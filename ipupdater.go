package ipupdater

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// ip-checking service - must return a body with the IP only
	checkipURL = "https://checkip.amazonaws.com"

	// The amount of time to wait after receiving a server error
	serverErrorWaitTime = 3 * time.Second
)

// Updater -
type Updater struct {
	domain string
	apikey string
	url    string
	ip     net.IP
}

// New creates an updater
func New(domain, apikey, endpoint string, ip net.IP) *Updater {
	return &Updater{
		domain: domain,
		apikey: apikey,
		ip:     ip,
		url:    endpoint,
	}
}

// https://help.dyn.com/remote-access-api/return-codes/
const (
	// Update Complete statuses
	//
	// Note that, for confirmation purposes, good and nochg messages will be
	// followed by the IP address that the hostname was updated to. This value
	// will be separated from the return code by a space.
	StatusGood     = "good"
	StatusNoChange = "nochg" // maybe backoff from this one exponentially?

	// Hostname-related errors (must stop immediately)
	StatusNotFQDN = "notfqdn" // The hostname specified is not a fully-qualified domain name
	StatusNoHost  = "nohost"  // The hostname specified does not exist in this user account
	StatusNumHost = "numhost" // Too many hosts specified in an update
	StatusAbuse   = "abuse"   // The hostname specified is blocked for update abuse.

	// Account-Related Errors (must stop immediately)
	StatusBadAuth = "badauth" // The username and password pair do not match a real user.

	// Agent-Related Errors
	StatusBadAgent = "badagent" // The user agent was not sent or HTTP method is not permitted

	// Server Error Conditions - The client must not resume updating until 30 minutes have passed
	StatusDNSErr   = "dnserr"   // DNS error encountered
	Status911      = "911"      // There is a problem or scheduled maintenance on our side.
	StatusInterval = "interval" // Rate limiting - interval too tight
)

// Update -
func (u *Updater) Update(ctx context.Context) (net.IP, error) {
	success := false

	done := timeOp("Update", u.domain)
	defer func(ctx context.Context) {
		done(ctx, success)
	}(ctx)

	data := url.Values{
		"hostname": []string{u.domain},
		"password": []string{u.apikey},
	}
	if u.ip != nil {
		data["myip"] = []string{u.ip.String()}
	}

	client := createHTTPClient(u.url)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.url, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(req)
	if err != nil {
		updateErrorsMetric.WithLabelValues(u.domain, err.Error()).Inc()

		return nil, fmt.Errorf("failed to update: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode > 299 {
		updateErrorsMetric.WithLabelValues(u.domain, res.Status).Inc()

		return nil, fmt.Errorf("couldn't update IP for %s: %s", u.domain, res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		updateErrorsMetric.WithLabelValues(u.domain, "readfailed").Inc()

		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	// u.log.Debug().Str("body", string(body)).Msg("Update - received body")

	status, ip, err := parseBody(body)
	if err != nil {
		updateErrorsMetric.WithLabelValues(u.domain, status).Inc()

		return nil, err
	}

	if status == StatusNoChange {
		if u.ip != nil && !ip.Equal(u.ip) {
			updateErrorsMetric.WithLabelValues(u.domain, "unexpected IP").Inc()

			return nil, errors.Errorf("unexpected IP on remote end: %s - expected %s (status %s)", ip, u.ip, status)
		}
	}

	if status == StatusGood {
		lastUpdatedMetric.WithLabelValues(u.domain).SetToCurrentTime()
	}

	u.ip = ip
	currentIPMetric.WithLabelValues(u.domain, ip.String()).Set(1)
	updatesMetric.WithLabelValues(u.domain, status).Inc()

	success = true

	// u.log.Debug().IPAddr("ip", ip).Str("status", status).Msg("Update")

	return nil, nil
}

// CheckIP gets the current IP (requires a working internet connection)
func (u *Updater) CheckIP(ctx context.Context) (net.IP, error) {
	success := false

	done := timeOp("CheckIP", u.domain)
	defer func(ctx context.Context) {
		done(ctx, success)
	}(ctx)

	client := createHTTPClient(checkipURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkipURL, nil)
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		checkErrorsMetric.WithLabelValues(u.domain, err.Error()).Inc()

		return nil, err
	}

	defer res.Body.Close()

	if res.StatusCode > 299 {
		checkErrorsMetric.WithLabelValues(u.domain, res.Status).Inc()

		return nil, errors.Errorf("couldn't check IP: %s", res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		checkErrorsMetric.WithLabelValues(u.domain, "readfailed").Inc()

		return nil, errors.Wrap(err, "failed to read response body")
	}

	ip := net.ParseIP(strings.TrimSpace(string(body)))
	if ip == nil {
		checkErrorsMetric.WithLabelValues(u.domain, "invalidip").Inc()

		return nil, errors.Errorf("failed to parse IP: %s", body)
	}

	success = true

	checksMetric.WithLabelValues(u.domain).Inc()

	return ip, nil
}

// Lookup returns the resolved IP from the domain
func (u *Updater) Lookup(ctx context.Context) (net.IP, error) {
	success := false

	done := timeOp("Lookup", u.domain)
	defer func(ctx context.Context) {
		done(ctx, success)
	}(ctx)

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, u.domain)
	if err != nil {
		lookupErrorsMetric.WithLabelValues(u.domain, err.Error()).Inc()

		return nil, errors.Wrap(err, "failed to lookup IP")
	}

	if len(ips) == 0 {
		lookupErrorsMetric.WithLabelValues(u.domain, "nonefound").Inc()

		return nil, errors.New("DNS lookup returned no IPs")
	}

	// if len(ips) > 1 {
	// 	// u.log.Warn().Int("ips_found", len(ips)).Msg("too many IPs found, only one expected! Picking the first")
	// }

	success = true

	lookupsMetric.WithLabelValues(u.domain).Inc()

	return ips[0].IP, nil
}

// Loop -
func (u *Updater) Loop(ctx context.Context, interval time.Duration) error {
	tick := time.NewTicker(interval)

	for {
		select {
		case <-tick.C:
			ip, err := u.CheckIP(ctx)
			if err != nil {
				// u.log.Error().Err(err).Msg("failed to check IP")
				continue
			}

			dnsIP, err := u.Lookup(ctx)
			if err != nil {
				// u.log.Error().Err(err).Msg("failed to lookup IP")
				continue
			}

			if u.ip == nil || !u.ip.Equal(ip) || !dnsIP.Equal(ip) {
				_, err = u.Update(ctx)
				// if err != nil {
				// 	// u.log.Error().Err(err).Msg("failed to update")
				// }

				switch err.(type) {
				case ClientError:
					tick.Stop()

					return err
				case ServerError:
					time.Sleep(serverErrorWaitTime)
				}
			}
		case <-ctx.Done():
			tick.Stop()

			return nil
		}
	}
}

func parseBody(body []byte) (status string, ip net.IP, err error) {
	parts := strings.SplitN(strings.TrimSpace(string(body)), " ", 2)
	status = parts[0]

	if len(parts) == 2 {
		ip = net.ParseIP(parts[1])
	}

	switch status {
	case StatusGood, StatusNoChange:
		if len(parts) != 2 {
			return "", nil, errors.Errorf("malformed response body %s", body)
		}

		return status, ip, nil
	case StatusNotFQDN, StatusNoHost, StatusNumHost, StatusAbuse, StatusBadAuth, StatusBadAgent:
		return status, nil, ClientError{status}
	case StatusDNSErr, Status911, StatusInterval:
		return status, nil, ServerError{status}
	default:
		return status, ip, errors.Errorf("unexpected status from %s", body)
	}
}

func createHTTPClient(url string) *http.Client {
	rt := promhttp.InstrumentRoundTripperDuration(
		httpClientDurationHist.MustCurryWith(prometheus.Labels{"url": url}),
		http.DefaultTransport,
	)

	client := &http.Client{
		Transport: rt,
	}

	return client
}

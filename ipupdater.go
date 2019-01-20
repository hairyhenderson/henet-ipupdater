package ipupdater

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/context/ctxhttp"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	// URLTemplate the template string for building the URL to update the IP
	// URLTemplate = "https://{{.Domain}}:{{.APIKey}}@dyn.dns.he.net/nic/update?hostname={{.Domain}}"

	// the default URL
	updateURL = "https://dyn.dns.he.net/nic/update"
)

// Updater -
type Updater struct {
	domain string
	apikey string
	ip     net.IP
	url    string
	log    zerolog.Logger
}

// New creates an updater
func New(domain, apikey string, ip net.IP) *Updater {
	l := log.With().Str("domain", domain).Logger()
	return &Updater{
		domain: domain,
		apikey: apikey,
		ip:     ip,
		url:    updateURL, // TODO: make this overridable
		log:    l,
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
	StatusDNSErr = "dnserr" // DNS error encountered
	Status911    = "911"    // There is a problem or scheduled maintenance on our side.
)

// Update -
func (u *Updater) Update(ctx context.Context) (net.IP, error) {
	u.log.Debug().Msg("Update")
	client := &http.Client{}
	data := url.Values{
		"hostname": []string{u.domain},
		"password": []string{u.apikey},
	}
	if u.ip != nil {
		data["myip"] = []string{u.ip.String()}
	}
	res, err := ctxhttp.PostForm(ctx, client, u.url, data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to update")
	}
	if res.StatusCode > 299 {
		return nil, errors.Errorf("couldn't update IP for %s: %s", u.domain, res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}
	status, ip, err := parseBody(body)
	if err != nil {
		return nil, err
	}
	if status == StatusNoChange {
		u.log.Debug().IPAddr("ip", ip).Msg("No change")
	}
	return nil, nil
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
	case StatusDNSErr, Status911:
		return status, nil, ServerError{status}
	default:
		return status, ip, errors.Errorf("unexpected status from %s", body)
	}
}

// Loop -
func (u *Updater) Loop(ctx context.Context, interval time.Duration) error {
	defer ret()
	tick := time.NewTicker(interval)
	for {
		select {
		case <-tick.C:
			_, err := u.Update(ctx)
			if err != nil {
				u.log.Error().Err(err).Msg(err.Error())
			}
			switch err.(type) {
			case ClientError:
				tick.Stop()
				return err
			case ServerError:
				time.Sleep(3 * time.Second)
			}
		case <-ctx.Done():
			tick.Stop()
			return nil
		}
	}
}

func ret() {
	log.Debug().Msg("Returning!")
}

// ClientError - an error that's the client's fault (probably bad domain or key)
type ClientError struct {
	status string
}

func (e ClientError) Error() string {
	return "client error: " + e.status
}

// ServerError - a server-side error, usually temporary
type ServerError struct {
	status string
}

func (e ServerError) Error() string {
	return "server error: " + e.status
}

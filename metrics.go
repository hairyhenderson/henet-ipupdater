package ipupdater

import (
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/hairyhenderson/henet-ipupdater/version"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// opDurationHistogram    *prometheus.HistogramVec
	// opDurationSummary      *prometheus.SummaryVec
	// updatesMetric          *prometheus.CounterVec
	// updateErrorsMetric     *prometheus.CounterVec
	// checksMetric           *prometheus.CounterVec
	// checkErrorsMetric      *prometheus.CounterVec
	// lookupsMetric          *prometheus.CounterVec
	// lookupErrorsMetric     *prometheus.CounterVec
	// currentIPMetric        *prometheus.GaugeVec
	// lastUpdatedMetric      *prometheus.GaugeVec
	// intervalMetric         prometheus.Gauge
	// httpClientDurationHist *prometheus.HistogramVec
	// httpClientDurationSumm *prometheus.SummaryVec

	labelNames = []string{"domain", "status"}
	ns         = "ipupdater"

	opDurationHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns,
		Name:      "op_duration_seconds",
		Buckets:   prometheus.DefBuckets,
	}, []string{"op", "domain", "success"})
	opDurationSummary = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  ns,
		Name:       "op_duration_quantile_seconds",
		Objectives: map[float64]float64{0.1: 0.1, 0.5: 0.05, 0.9: 0.01, 0.99: 0.001, 0.999: 0.001},
		MaxAge:     1 * time.Hour,
	}, []string{"op", "domain", "success"})

	updatesMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "updates_total",
		Help:      "the number of updates completed, labeled by domain and status",
	}, labelNames)
	updateErrorsMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "update_errors_total",
		Help:      "the number of update errors, labeled by domain and status",
	}, labelNames)

	checksMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "checks_total",
		Help:      "the number of IP checks completed, labeled by domain",
	}, []string{"domain"})
	checkErrorsMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "check_errors_total",
		Help:      "the number of IP check errors, labeled by domain and reason",
	}, []string{"domain", "reason"})

	lookupsMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "lookups_total",
		Help:      "the number of DNS lookups completed, labeled by domain",
	}, []string{"domain"})
	lookupErrorsMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns,
		Name:      "lookup_errors_total",
		Help:      "the number of DNS lookup errors, labeled by domain and reason",
	}, []string{"domain", "reason"})

	currentIPMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: ns,
		Name:      "current_ip_info",
		Help:      "the current IP",
	}, []string{"domain", "ip"})

	lastUpdatedMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: ns,
		Name:      "last_updated_seconds",
		Help:      "the time the domain was last updated (with a 'good' status)",
	}, []string{"domain"})

	intervalMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: ns,
		Subsystem: "config",
		Name:      "interval_seconds",
		Help:      "the configured interval between checks",
	})

	httpClientDurationHist = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "httpclient",
		Name:      "request_duration_seconds",
		Buckets:   prometheus.DefBuckets,
	}, []string{"url", "code", "method"})
	httpClientDurationSumm = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  "httpclient",
		Name:       "request_duration_quantile_seconds",
		Objectives: map[float64]float64{0.1: 0.1, 0.5: 0.05, 0.9: 0.01, 0.99: 0.001, 0.999: 0.001},
		MaxAge:     1 * time.Hour,
	}, []string{"url", "code", "method"})

	buildInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: ns,
		Name:      "build_info",
		Help: fmt.Sprintf(
			"A metric with a constant '1' value labeled by version, revision, branch, and goversion from which %s was built.",
			ns,
		),
	}, []string{"branch", "goversion", "revision", "version"})
)

func init() {
	buildInfo.WithLabelValues(version.Branch, runtime.Version(), version.GitCommit, version.Version).Set(1)
}

func timeOp(op, domain string) func(success bool) {
	start := time.Now()
	return func(success bool) {
		duration := time.Now().Sub(start).Seconds()
		opDurationHistogram.WithLabelValues(op, domain, strconv.FormatBool(success)).Observe(duration)
		opDurationSummary.WithLabelValues(op, domain, strconv.FormatBool(success)).Observe(duration)
	}
}

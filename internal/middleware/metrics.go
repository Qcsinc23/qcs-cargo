package middleware

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
)

var (
	metricsOnce sync.Once

	httpRequestsTotal *prometheus.CounterVec
	httpRequestDurSec *prometheus.HistogramVec
	httpRequestsInFly prometheus.Gauge

	// DEF-005: per-job liveness gauge so missed runs are observable.
	dailyJobLastSuccess *prometheus.GaugeVec
	dailyJobRunsTotal   *prometheus.CounterVec
	dailyJobPanicsTotal *prometheus.CounterVec

	// DEF-003 + INC-001: outbound email reliability counter.
	emailSendFailuresTotal *prometheus.CounterVec

	// Pass 2.5 HIGH-10: number of outbound_emails rows reaped from stuck
	// 'in_progress' state per job run. Non-zero means the previous run
	// crashed mid-dispatch.
	outboundEmailReapedTotal *prometheus.CounterVec
)

func registerMetrics() {
	httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "qcs",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Count of HTTP requests by method/route/status class.",
	}, []string{"method", "route", "status_class"})

	httpRequestDurSec = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "qcs",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "Duration of HTTP requests by method/route/status class.",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"method", "route", "status_class"})

	httpRequestsInFly = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "qcs",
		Subsystem: "http",
		Name:      "requests_in_flight",
		Help:      "Number of in-flight HTTP requests.",
	})

	dailyJobLastSuccess = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "qcs",
		Subsystem: "jobs",
		Name:      "last_successful_run_unix_seconds",
		Help:      "Unix timestamp of the last successful run of each named daily job.",
	}, []string{"job"})

	dailyJobRunsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "qcs",
		Subsystem: "jobs",
		Name:      "runs_total",
		Help:      "Total daily job runs by name and outcome (success|error).",
	}, []string{"job", "outcome"})

	dailyJobPanicsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "qcs",
		Subsystem: "jobs",
		Name:      "panics_total",
		Help:      "Daily job goroutine panics by name; non-zero indicates the supervisor recovered the loop.",
	}, []string{"job"})

	emailSendFailuresTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "qcs",
		Subsystem: "email",
		Name:      "send_failures_total",
		Help:      "Outbound email send failures by template and reason.",
	}, []string{"template", "reason"})

	outboundEmailReapedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "qcs",
		Subsystem: "jobs",
		Name:      "outbound_email_reaped_total",
		Help:      "Outbound email rows reaped from stuck in_progress state.",
	}, []string{})

	prometheus.MustRegister(
		httpRequestsTotal, httpRequestDurSec, httpRequestsInFly,
		dailyJobLastSuccess, dailyJobRunsTotal, dailyJobPanicsTotal,
		emailSendFailuresTotal, outboundEmailReapedTotal,
	)
}

// EnsureMetricsRegistered allows callers outside the HTTP middleware (jobs,
// startup code) to read or update metrics without first taking an HTTP
// request. Safe to call multiple times.
func EnsureMetricsRegistered() {
	metricsOnce.Do(registerMetrics)
}

// RecordDailyJobRun marks a job run with the given outcome. outcome should be
// "success" or "error". On success it also updates the last-success gauge.
func RecordDailyJobRun(job, outcome string) {
	EnsureMetricsRegistered()
	dailyJobRunsTotal.WithLabelValues(job, outcome).Inc()
	if outcome == "success" {
		dailyJobLastSuccess.WithLabelValues(job).Set(float64(time.Now().Unix()))
	}
}

// RecordDailyJobPanic increments the panic counter for the named job.
func RecordDailyJobPanic(job string) {
	EnsureMetricsRegistered()
	dailyJobPanicsTotal.WithLabelValues(job).Inc()
}

// LastSuccessfulJobRun returns the Unix timestamp recorded in the gauge, or 0
// if the job has never reported a success in this process.
func LastSuccessfulJobRun(job string) int64 {
	EnsureMetricsRegistered()
	g, err := dailyJobLastSuccess.GetMetricWithLabelValues(job)
	if err != nil {
		return 0
	}
	var m dto.Metric
	if err := g.Write(&m); err != nil || m.Gauge == nil || m.Gauge.Value == nil {
		return 0
	}
	return int64(*m.Gauge.Value)
}

// RecordEmailSendFailure increments the email send failure counter.
func RecordEmailSendFailure(template, reason string) {
	EnsureMetricsRegistered()
	emailSendFailuresTotal.WithLabelValues(template, reason).Inc()
}

// RecordOutboundEmailReaped increments the reaped counter by n.
func RecordOutboundEmailReaped(n int) {
	if n <= 0 {
		return
	}
	EnsureMetricsRegistered()
	outboundEmailReapedTotal.WithLabelValues().Add(float64(n))
}

func normalizedRoute(c *fiber.Ctx) string {
	if r := c.Route(); r != nil {
		if path := strings.TrimSpace(r.Path); path != "" {
			return path
		}
	}
	return "unmatched"
}

// MetricsMiddleware records request count and latency for Prometheus scraping.
func MetricsMiddleware(c *fiber.Ctx) error {
	metricsOnce.Do(registerMetrics)

	start := time.Now()
	httpRequestsInFly.Inc()
	defer httpRequestsInFly.Dec()

	err := c.Next()
	statusCode := c.Response().StatusCode()
	if statusCode == 0 {
		statusCode = fiber.StatusOK
	}

	statusClass := fmt.Sprintf("%dxx", statusCode/100)
	httpRequestsTotal.WithLabelValues(c.Method(), normalizedRoute(c), statusClass).Inc()
	httpRequestDurSec.WithLabelValues(c.Method(), normalizedRoute(c), statusClass).Observe(time.Since(start).Seconds())
	return err
}

// RequestMetrics is an alias kept for compatibility with previous wiring.
func RequestMetrics(c *fiber.Ctx) error {
	return MetricsMiddleware(c)
}

// MetricsHandler serves Prometheus metrics in text exposition format.
func MetricsHandler(c *fiber.Ctx) error {
	metricsOnce.Do(registerMetrics)
	return adaptor.HTTPHandler(promhttp.Handler())(c)
}

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
)

var (
	metricsOnce sync.Once

	httpRequestsTotal *prometheus.CounterVec
	httpRequestDurSec *prometheus.HistogramVec
	httpRequestsInFly prometheus.Gauge
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

	prometheus.MustRegister(httpRequestsTotal, httpRequestDurSec, httpRequestsInFly)
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

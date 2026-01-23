package middleware

import (
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricsOnce sync.Once

	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"service", "method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"service", "method", "path", "status"},
	)

	httpInFlightRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_in_flight_requests",
			Help: "Current number of in-flight HTTP requests",
		},
		[]string{"service"},
	)
)

func RegisterMetrics() {
	metricsOnce.Do(func() {
		prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, httpInFlightRequests)
	})
}

func Metrics(serviceName string) gin.HandlerFunc {
	RegisterMetrics()
	return func(c *gin.Context) {
		start := time.Now()
		httpInFlightRequests.WithLabelValues(serviceName).Inc()

		defer func() {
			status := strconv.Itoa(c.Writer.Status())
			path := c.FullPath()
			if path == "" {
				path = c.Request.URL.Path
			}
			duration := time.Since(start).Seconds()

			httpInFlightRequests.WithLabelValues(serviceName).Dec()
			httpRequestsTotal.WithLabelValues(serviceName, c.Request.Method, path, status).Inc()
			httpRequestDuration.WithLabelValues(serviceName, c.Request.Method, path, status).Observe(duration)
		}()

		c.Next()
	}
}

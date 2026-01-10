package observability

import (
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "user_http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "route", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "user_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"route"},
	)
	auditEventsPublishedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "user_audit_events_published_total",
			Help: "Total number of audit events published.",
		},
		[]string{"event_name"},
	)
	amqpPublishErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "user_amqp_publish_errors_total",
			Help: "Total number of AMQP publish errors.",
		},
	)
	metricsOnce sync.Once
)

func InitMetrics(reg prometheus.Registerer) {
	metricsOnce.Do(func() {
		reg.MustRegister(httpRequestsTotal, httpRequestDuration, auditEventsPublishedTotal, amqpPublishErrorsTotal)
	})
}

func RecordHTTPRequest(method, route string, status int, duration time.Duration) {
	if route == "" {
		route = "unknown"
	}
	httpRequestsTotal.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	httpRequestDuration.WithLabelValues(route).Observe(duration.Seconds())
}

func IncAuditEventPublished(eventName string) {
	if eventName == "" {
		eventName = "unknown"
	}
	auditEventsPublishedTotal.WithLabelValues(eventName).Inc()
}

func IncAMQPPublishError() {
	amqpPublishErrorsTotal.Inc()
}

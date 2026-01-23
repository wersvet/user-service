package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

var (
	friendMetricsOnce sync.Once

	friendRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "friend_requests_total",
			Help: "Total number of friend request attempts",
		},
		[]string{"status"},
	)

	friendAcceptsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "friend_accepts_total",
			Help: "Total number of friend request accept attempts",
		},
		[]string{"status"},
	)

	friendRejectsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "friend_rejects_total",
			Help: "Total number of friend request reject attempts",
		},
		[]string{"status"},
	)
)

func RegisterFriendMetrics() {
	friendMetricsOnce.Do(func() {
		prometheus.MustRegister(friendRequestsTotal, friendAcceptsTotal, friendRejectsTotal)
	})
}

func IncFriendRequest(status string) {
	RegisterFriendMetrics()
	friendRequestsTotal.WithLabelValues(status).Inc()
}

func IncFriendAccept(status string) {
	RegisterFriendMetrics()
	friendAcceptsTotal.WithLabelValues(status).Inc()
}

func IncFriendReject(status string) {
	RegisterFriendMetrics()
	friendRejectsTotal.WithLabelValues(status).Inc()
}

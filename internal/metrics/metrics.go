// Package metrics defines and registers the Prometheus collectors used across
// the application.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Collectors groups the application metrics.
type Collectors struct {
	HTTPRequests  *prometheus.CounterVec
	HTTPDuration  *prometheus.HistogramVec
	JobsExecuted  *prometheus.CounterVec
	JobDuration   prometheus.Histogram
	LockConflicts prometheus.Counter
	ActiveNodes   prometheus.Gauge
}

// New registers and returns the application collectors.
func New() *Collectors {
	return &Collectors{
		HTTPRequests: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests by method, route and status.",
		}, []string{"method", "route", "status"}),
		HTTPDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
		JobsExecuted: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "jobs_executed_total",
			Help: "Total number of job executions by outcome.",
		}, []string{"status"}),
		JobDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "job_execution_duration_seconds",
			Help:    "Job execution duration in seconds.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
		}),
		LockConflicts: promauto.NewCounter(prometheus.CounterOpts{
			Name: "lock_conflicts_total",
			Help: "Number of times a job lock was already held by another node.",
		}),
		ActiveNodes: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "scheduler_up",
			Help: "Whether this scheduler node is running (1) or not (0).",
		}),
	}
}

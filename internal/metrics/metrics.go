package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// SentTotal counts delivered emails by status (success|error).
	SentTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mailer_sent_total",
		Help: "Number of emails handled by the mailer",
	}, []string{"status"})

	// Duration observes the time taken to deliver a single email.
	Duration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "mailer_duration_seconds",
		Help:    "Email delivery duration in seconds",
		Buckets: prometheus.DefBuckets,
	})
)

func init() {
	prometheus.MustRegister(SentTotal, Duration)
}

func SentSuccess() { SentTotal.WithLabelValues("success").Inc() }

func SentError() { SentTotal.WithLabelValues("error").Inc() }
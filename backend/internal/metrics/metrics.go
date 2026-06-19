// Package metrics defines the Prometheus collectors shared across the HTTP API and
// Kafka consumers, plus an echo middleware for request latency.
package metrics

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// NotificationsTotal counts notifications by channel and terminal status
	// (queued, delivered, failed) — incremented at send time and at delivery-receipt time.
	NotificationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "notifyx_notifications_total",
		Help: "Total notifications processed, by channel and status.",
	}, []string{"channel", "status"})

	// RetriesTotal counts consumer retry attempts by channel.
	RetriesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "notifyx_retries_total",
		Help: "Total delivery retry attempts, by channel.",
	}, []string{"channel"})

	// DLQTotal counts messages that exhausted retries and landed in the DLQ, by channel.
	DLQTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "notifyx_dlq_total",
		Help: "Total messages sent to the dead-letter queue, by channel.",
	}, []string{"channel"})

	// ChannelSendDuration tracks how long each channel provider call (Resend/FCM/Fast2SMS)
	// takes, so slow providers show up before they cause consumer-lag.
	ChannelSendDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "notifyx_channel_send_duration_seconds",
		Help:    "Time spent calling the channel provider, by channel.",
		Buckets: prometheus.DefBuckets,
	}, []string{"channel"})

	// HTTPRequestDuration tracks API latency by route and status code.
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "notifyx_http_request_duration_seconds",
		Help:    "HTTP request latency, by route and status code.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route", "status"})
)

// EchoMiddleware records HTTPRequestDuration for every request. Uses c.Path() (the
// route pattern, e.g. "/api/v1/tenants/:id") rather than c.Request().URL.Path, so
// distinct tenant/notification IDs don't each get their own label series.
func EchoMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			HTTPRequestDuration.WithLabelValues(
				c.Request().Method,
				c.Path(),
				strconv.Itoa(c.Response().Status),
			).Observe(time.Since(start).Seconds())
			return err
		}
	}
}

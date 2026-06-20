package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rohit-bagade/notifyx/internal/domain"
)

// AnalyticsRepository runs the read-only aggregate queries behind the Phase 10
// analytics endpoint. It reads from notifications and dlq_messages directly rather
// than going through NotificationRepository/DLQRepository, since these are
// aggregate queries, not row-shaped CRUD.
// AnalyticsRepositoryInterface is the seam used by handlers so they can be unit-tested
// against a mock instead of a real Postgres connection.
type AnalyticsRepositoryInterface interface {
	Summary(ctx context.Context, tenantID uuid.UUID, from, to time.Time) (*domain.AnalyticsSummary, error)
	ChannelBreakdown(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]*domain.ChannelBreakdown, error)
	DLQTrend(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]*domain.DLQTrendPoint, error)
}

type AnalyticsRepository struct {
	pool Executor
}

var _ AnalyticsRepositoryInterface = (*AnalyticsRepository)(nil)

func NewAnalyticsRepository(pool Executor) *AnalyticsRepository {
	return &AnalyticsRepository{pool: pool}
}

// Summary returns total sent/delivered/failed counts for tenantID within [from, to].
func (r *AnalyticsRepository) Summary(ctx context.Context, tenantID uuid.UUID, from, to time.Time) (*domain.AnalyticsSummary, error) {
	var s domain.AnalyticsSummary
	err := r.pool.QueryRow(ctx,
		`SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = $4),
			COUNT(*) FILTER (WHERE status = $5)
		 FROM notifications
		 WHERE tenant_id = $1 AND created_at >= $2 AND created_at <= $3`,
		tenantID, from, to, string(domain.StatusDelivered), string(domain.StatusFailed),
	).Scan(&s.TotalSent, &s.TotalDelivered, &s.TotalFailed)
	if err != nil {
		return nil, fmt.Errorf("analytics summary: %w", err)
	}
	return &s, nil
}

// ChannelBreakdown returns per-channel sent/delivered/failed counts and delivery rate
// for tenantID within [from, to]. Only channels with at least one notification appear.
func (r *AnalyticsRepository) ChannelBreakdown(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]*domain.ChannelBreakdown, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT
			channel,
			COUNT(*) AS sent,
			COUNT(*) FILTER (WHERE status = $4) AS delivered,
			COUNT(*) FILTER (WHERE status = $5) AS failed
		 FROM notifications
		 WHERE tenant_id = $1 AND created_at >= $2 AND created_at <= $3
		 GROUP BY channel
		 ORDER BY channel`,
		tenantID, from, to, string(domain.StatusDelivered), string(domain.StatusFailed),
	)
	if err != nil {
		return nil, fmt.Errorf("analytics channel breakdown: %w", err)
	}
	defer rows.Close()

	var breakdown []*domain.ChannelBreakdown
	for rows.Next() {
		var b domain.ChannelBreakdown
		var channel string
		if err := rows.Scan(&channel, &b.Sent, &b.Delivered, &b.Failed); err != nil {
			return nil, fmt.Errorf("scan channel breakdown: %w", err)
		}
		b.Channel = domain.Channel(channel)
		if b.Sent > 0 {
			b.DeliveryRate = float64(b.Delivered) / float64(b.Sent)
		}
		breakdown = append(breakdown, &b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channel breakdown: %w", err)
	}
	return breakdown, nil
}

// DLQTrend returns one row per UTC day in [from, to] with the count of DLQ messages
// for notifications belonging to tenantID, oldest first. Days with zero DLQ messages
// are included so the chart has no gaps.
func (r *AnalyticsRepository) DLQTrend(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]*domain.DLQTrendPoint, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT g.day::date, COALESCE(c.count, 0)
		 FROM generate_series(date_trunc('day', $2::timestamptz), date_trunc('day', $3::timestamptz), interval '1 day') AS g(day)
		 LEFT JOIN (
			SELECT date_trunc('day', d.created_at) AS day, COUNT(*) AS count
			FROM dlq_messages d
			JOIN notifications n ON n.id = d.notification_id
			WHERE n.tenant_id = $1 AND d.created_at >= $2 AND d.created_at <= $3
			GROUP BY date_trunc('day', d.created_at)
		 ) c ON c.day = g.day
		 ORDER BY g.day`,
		tenantID, from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("analytics dlq trend: %w", err)
	}
	defer rows.Close()

	var trend []*domain.DLQTrendPoint
	for rows.Next() {
		var p domain.DLQTrendPoint
		if err := rows.Scan(&p.Date, &p.Count); err != nil {
			return nil, fmt.Errorf("scan dlq trend point: %w", err)
		}
		trend = append(trend, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dlq trend: %w", err)
	}
	return trend, nil
}

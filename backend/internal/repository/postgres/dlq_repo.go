package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
)

// DLQRepositoryInterface is the seam used by kafka consumers so they can be unit-tested
// against a mock instead of a real Postgres connection.
type DLQRepositoryInterface interface {
	Create(ctx context.Context, msg *kafkatypes.Message, errMsg string) error
}

// DLQRepository writes failed messages to the dlq_messages table.
type DLQRepository struct {
	pool *pgxpool.Pool
}

var _ DLQRepositoryInterface = (*DLQRepository)(nil)

func NewDLQRepository(pool *pgxpool.Pool) *DLQRepository {
	return &DLQRepository{pool: pool}
}

// Create persists a message that exhausted all delivery retries.
// errMsg is the last error returned by the channel handler.
func (r *DLQRepository) Create(ctx context.Context, msg *kafkatypes.Message, errMsg string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO dlq_messages (notification_id, channel, error, attempts, last_tried_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		msg.NotificationID,
		string(msg.Channel),
		errMsg,
		3, // always maxRetries by the time we reach DLQ
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert dlq message: %w", err)
	}
	return nil
}

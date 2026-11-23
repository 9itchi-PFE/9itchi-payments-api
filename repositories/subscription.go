package repositories

import (
	"context"

	"9itchi-payments-api/models/db"

	"github.com/jackc/pgx/v5"
)

const subscriptionColumns = `id, user_id, plan_id, promo_code_id, status, amount, starts_at, ends_at, created_at, updated_at`

// SubscriptionRepository is the only layer that touches PostgreSQL for subscription data.
type SubscriptionRepository interface {
	// GetActiveByUser returns the user's active, unexpired subscription (ErrNotFound if none).
	GetActiveByUser(ctx context.Context, userID string) (*db.Subscription, error)
}

type subscriptionRepository struct {
	store *Store
}

func NewSubscriptionRepository(store *Store) SubscriptionRepository {
	return &subscriptionRepository{store: store}
}

func (r *subscriptionRepository) GetActiveByUser(ctx context.Context, userID string) (*db.Subscription, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT `+subscriptionColumns+` FROM subscriptions
		 WHERE user_id = $1 AND status = 'active' AND ends_at > now()`, userID)
	if err != nil {
		return nil, MapError(err)
	}
	s, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Subscription])
	return s, MapError(err)
}

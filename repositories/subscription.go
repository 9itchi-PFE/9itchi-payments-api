package repositories

import (
	"context"
	"time"

	"9itchi-payments-api/models/db"

	"github.com/jackc/pgx/v5"
)

const subscriptionColumns = `id, user_id, plan_id, promo_code_id, status, amount, starts_at, ends_at, stopped_at, created_at, updated_at`

// SubscriptionRepository is the only layer that touches PostgreSQL for subscription data.
type SubscriptionRepository interface {
	// GetActiveByUser returns the user's active, unexpired subscription (ErrNotFound if none).
	GetActiveByUser(ctx context.Context, userID string) (*db.Subscription, error)
	// Create inserts a subscription (status active unless set); ErrConflict if the user already has an active one.
	Create(ctx context.Context, s *db.Subscription) (*db.Subscription, error)
	// Extend moves ends_at of a subscription (ErrNotFound if unknown). Paying again also resumes a stopped renewal.
	Extend(ctx context.Context, id string, endsAt time.Time) error
	// ExpireActive marks every active subscription of the user expired (also clears lapsed ones).
	ExpireActive(ctx context.Context, userID string) error
	// SetRenewal stops (renews=false) or resumes the renewal of the user's active subscription and returns it
	// (ErrNotFound if none). Access is untouched: the subscription stays active until ends_at.
	SetRenewal(ctx context.Context, userID string, renews bool) (*db.Subscription, error)
	// LastEnded returns the user's most recent subscription whose period is over (ErrNotFound if none).
	LastEnded(ctx context.Context, userID string) (*db.Subscription, error)
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

func (r *subscriptionRepository) Create(ctx context.Context, s *db.Subscription) (*db.Subscription, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`INSERT INTO subscriptions (user_id, plan_id, promo_code_id, status, amount, starts_at, ends_at)
		 VALUES ($1, $2, $3, COALESCE(NULLIF($4, ''), 'active'), $5, $6, $7) RETURNING `+subscriptionColumns,
		s.UserID, s.PlanID, s.PromoCodeID, s.Status, s.Amount, s.StartsAt, s.EndsAt)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Subscription])
	return out, MapError(err)
}

func (r *subscriptionRepository) Extend(ctx context.Context, id string, endsAt time.Time) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	tag, err := r.store.Conn(ctx).Exec(ctx,
		`UPDATE subscriptions SET ends_at = $2, stopped_at = NULL, updated_at = now() WHERE id = $1`, id, endsAt)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *subscriptionRepository) ExpireActive(ctx context.Context, userID string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	_, err := r.store.Conn(ctx).Exec(ctx,
		`UPDATE subscriptions SET status = 'expired', updated_at = now() WHERE user_id = $1 AND status = 'active'`, userID)
	return MapError(err)
}

func (r *subscriptionRepository) SetRenewal(ctx context.Context, userID string, renews bool) (*db.Subscription, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`UPDATE subscriptions SET stopped_at = CASE WHEN $2 THEN NULL ELSE COALESCE(stopped_at, now()) END, updated_at = now()
		 WHERE user_id = $1 AND status = 'active' AND ends_at > now() RETURNING `+subscriptionColumns, userID, renews)
	if err != nil {
		return nil, MapError(err)
	}
	s, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Subscription])
	return s, MapError(err)
}

func (r *subscriptionRepository) LastEnded(ctx context.Context, userID string) (*db.Subscription, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT `+subscriptionColumns+` FROM subscriptions
		 WHERE user_id = $1 AND ends_at <= now() AND status IN ('active', 'expired')
		 ORDER BY ends_at DESC, id DESC LIMIT 1`, userID)
	if err != nil {
		return nil, MapError(err)
	}
	s, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Subscription])
	return s, MapError(err)
}

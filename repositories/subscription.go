package repositories

import (
	"context"
	"fmt"
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
	// ClaimReminders marks and returns, at most limit at a time, the renewing subscriptions of a paid plan that need
	// the reminder of a stage and did not get it for their current period. ReminderDue: the period ends within
	// window. ReminderLapsed: it ended within window and the user has no running subscription. A subscription is
	// claimed once per period, even when several instances ask at the same time.
	ClaimReminders(ctx context.Context, stage string, window time.Duration, limit int) ([]db.RenewalReminder, error)
	// ReleaseReminder undoes a claim whose reminder could not be raised, so that it is tried again.
	ReleaseReminder(ctx context.Context, subscriptionID, stage string) error
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

// reminderQueries claim the subscriptions of one stage. The inner SELECT locks the rows it takes, so two instances
// never claim the same subscription.
var reminderQueries = map[string]string{
	db.ReminderDue: `
		UPDATE subscriptions s SET due_reminded_for = s.ends_at
		FROM plans p
		WHERE p.id = s.plan_id AND p.price > 0 AND s.id IN (
			SELECT id FROM subscriptions
			WHERE status = 'active' AND stopped_at IS NULL
			  AND ends_at > now() AND ends_at <= now() + make_interval(secs => $1)
			  AND due_reminded_for IS DISTINCT FROM ends_at
			ORDER BY ends_at LIMIT $2 FOR UPDATE SKIP LOCKED)
		RETURNING s.id, s.user_id, s.plan_id, p.name, p.price, s.ends_at`,
	db.ReminderLapsed: `
		UPDATE subscriptions s SET lapse_reminded_for = s.ends_at
		FROM plans p
		WHERE p.id = s.plan_id AND p.price > 0 AND s.id IN (
			SELECT l.id FROM subscriptions l
			WHERE l.status IN ('active', 'expired') AND l.stopped_at IS NULL
			  AND l.ends_at <= now() AND l.ends_at > now() - make_interval(secs => $1)
			  AND l.lapse_reminded_for IS DISTINCT FROM l.ends_at
			  AND NOT EXISTS (SELECT 1 FROM subscriptions a WHERE a.user_id = l.user_id AND a.status = 'active' AND a.ends_at > now())
			ORDER BY l.ends_at LIMIT $2 FOR UPDATE SKIP LOCKED)
		RETURNING s.id, s.user_id, s.plan_id, p.name, p.price, s.ends_at`,
}

func (r *subscriptionRepository) ClaimReminders(ctx context.Context, stage string, window time.Duration, limit int) ([]db.RenewalReminder, error) {
	query, ok := reminderQueries[stage]
	if !ok {
		return nil, fmt.Errorf("unknown reminder stage %q", stage)
	}
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx, query, window.Seconds(), limit)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.RenewalReminder])
	return out, MapError(err)
}

func (r *subscriptionRepository) ReleaseReminder(ctx context.Context, subscriptionID, stage string) error {
	column := "due_reminded_for"
	switch stage {
	case db.ReminderDue:
	case db.ReminderLapsed:
		column = "lapse_reminded_for"
	default:
		return fmt.Errorf("unknown reminder stage %q", stage)
	}
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	_, err := r.store.Conn(ctx).Exec(ctx, `UPDATE subscriptions SET `+column+` = NULL WHERE id = $1`, subscriptionID)
	return MapError(err)
}

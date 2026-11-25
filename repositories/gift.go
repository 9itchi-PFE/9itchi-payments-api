package repositories

import (
	"context"

	"9itchi-payments-api/models/db"

	"github.com/jackc/pgx/v5"
)

const giftColumns = `id, user_id, subscription_id, plan_id, days, given_by, period_start, period_end, created_at`

// GiftRepository is the only layer that touches PostgreSQL for the days the team gives.
type GiftRepository interface {
	// LockUser makes the gifts to one student wait for each other until the transaction ends, so that two admins
	// (or a double click) cannot give at the same instant. It must be called inside a transaction.
	LockUser(ctx context.Context, userID string) error
	Create(ctx context.Context, g *db.Gift) (*db.Gift, error)
	// ListByUser returns the gifts to a student, newest first.
	ListByUser(ctx context.Context, userID string, limit int) ([]db.Gift, error)
}

type giftRepository struct {
	store *Store
}

func NewGiftRepository(store *Store) GiftRepository {
	return &giftRepository{store: store}
}

func (r *giftRepository) LockUser(ctx context.Context, userID string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	_, err := r.store.Conn(ctx).Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('gift:' || $1::text, 0))`, userID)
	return MapError(err)
}

func (r *giftRepository) Create(ctx context.Context, g *db.Gift) (*db.Gift, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`INSERT INTO subscription_gifts (user_id, subscription_id, plan_id, days, given_by, period_start, period_end)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+giftColumns,
		g.UserID, g.SubscriptionID, g.PlanID, g.Days, g.GivenBy, g.PeriodStart, g.PeriodEnd)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Gift])
	return out, MapError(err)
}

func (r *giftRepository) ListByUser(ctx context.Context, userID string, limit int) ([]db.Gift, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT `+giftColumns+` FROM subscription_gifts WHERE user_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.Gift])
	return out, MapError(err)
}

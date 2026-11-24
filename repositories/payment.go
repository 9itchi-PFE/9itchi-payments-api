package repositories

import (
	"context"

	"9itchi-payments-api/models/db"

	"github.com/jackc/pgx/v5"
)

const paymentColumns = `id, user_id, plan_id, promo_code_id, provider, provider_ref, status, amount_millimes, discount_millimes, completed_at, created_at, updated_at`

// PaymentRepository is the only layer that touches PostgreSQL for payment data.
type PaymentRepository interface {
	// Create inserts a payment (status pending unless set) and returns it.
	Create(ctx context.Context, p *db.Payment) (*db.Payment, error)
	// SetProviderRef stores the gateway reference of a pending payment (ErrConflict if already used).
	SetProviderRef(ctx context.Context, id, ref string) error
	GetByProviderRef(ctx context.Context, provider, ref string) (*db.Payment, error)
	// MarkCompleted flips a pending payment to completed; applied=false when it was not pending (replay, failed, unknown).
	MarkCompleted(ctx context.Context, id string) (applied bool, err error)
	// MarkFailed flips a pending payment to failed.
	MarkFailed(ctx context.Context, id string) error
}

type paymentRepository struct {
	store *Store
}

func NewPaymentRepository(store *Store) PaymentRepository {
	return &paymentRepository{store: store}
}

func (r *paymentRepository) Create(ctx context.Context, p *db.Payment) (*db.Payment, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`INSERT INTO payments (user_id, plan_id, promo_code_id, provider, status, amount_millimes, discount_millimes)
		 VALUES ($1, $2, $3, $4, COALESCE(NULLIF($5, ''), 'pending'), $6, $7) RETURNING `+paymentColumns,
		p.UserID, p.PlanID, p.PromoCodeID, p.Provider, p.Status, p.AmountMillimes, p.DiscountMillimes)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Payment])
	return out, MapError(err)
}

func (r *paymentRepository) SetProviderRef(ctx context.Context, id, ref string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	tag, err := r.store.Conn(ctx).Exec(ctx,
		`UPDATE payments SET provider_ref = $2, updated_at = now() WHERE id = $1 AND status = 'pending'`, id, ref)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *paymentRepository) GetByProviderRef(ctx context.Context, provider, ref string) (*db.Payment, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT `+paymentColumns+` FROM payments WHERE provider = $1 AND provider_ref = $2`, provider, ref)
	if err != nil {
		return nil, MapError(err)
	}
	p, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Payment])
	return p, MapError(err)
}

func (r *paymentRepository) MarkCompleted(ctx context.Context, id string) (bool, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	tag, err := r.store.Conn(ctx).Exec(ctx,
		`UPDATE payments SET status = 'completed', completed_at = now(), updated_at = now()
		 WHERE id = $1 AND status = 'pending'`, id)
	if err != nil {
		return false, MapError(err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *paymentRepository) MarkFailed(ctx context.Context, id string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	_, err := r.store.Conn(ctx).Exec(ctx,
		`UPDATE payments SET status = 'failed', updated_at = now() WHERE id = $1 AND status = 'pending'`, id)
	return MapError(err)
}

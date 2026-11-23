package repositories

import (
	"context"

	"9itchi-payments-api/models/db"

	"github.com/jackc/pgx/v5"
)

const promoColumns = `id, code, type, value, valid_from, valid_until, usage_limit, used_count, plan_ids, created_at, updated_at`

// PromoRepository is the only layer that touches PostgreSQL for promo data.
type PromoRepository interface {
	List(ctx context.Context) ([]db.PromoCode, error)
	GetByID(ctx context.Context, id string) (*db.PromoCode, error)
	GetByCode(ctx context.Context, code string) (*db.PromoCode, error)
	Create(ctx context.Context, p *db.PromoCode) (*db.PromoCode, error)
	Update(ctx context.Context, id string, p *db.PromoCode) (*db.PromoCode, error)
	Delete(ctx context.Context, id string) error
	// IncrementUsage atomically bumps used_count; applied=false when the usage limit is reached (or id unknown).
	IncrementUsage(ctx context.Context, id string) (applied bool, err error)
}

type promoRepository struct {
	store *Store
}

func NewPromoRepository(store *Store) PromoRepository {
	return &promoRepository{store: store}
}

func (r *promoRepository) List(ctx context.Context) ([]db.PromoCode, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx, `SELECT `+promoColumns+` FROM promo_codes ORDER BY created_at DESC, code`)
	if err != nil {
		return nil, MapError(err)
	}
	promos, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.PromoCode])
	return promos, MapError(err)
}

func (r *promoRepository) GetByID(ctx context.Context, id string) (*db.PromoCode, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx, `SELECT `+promoColumns+` FROM promo_codes WHERE id = $1`, id)
	if err != nil {
		return nil, MapError(err)
	}
	p, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.PromoCode])
	return p, MapError(err)
}

func (r *promoRepository) GetByCode(ctx context.Context, code string) (*db.PromoCode, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx, `SELECT `+promoColumns+` FROM promo_codes WHERE code = $1`, code)
	if err != nil {
		return nil, MapError(err)
	}
	p, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.PromoCode])
	return p, MapError(err)
}

func (r *promoRepository) Create(ctx context.Context, p *db.PromoCode) (*db.PromoCode, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`INSERT INTO promo_codes (code, type, value, valid_from, valid_until, usage_limit, plan_ids)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+promoColumns,
		p.Code, p.Type, p.Value, p.ValidFrom, p.ValidUntil, p.UsageLimit, p.PlanIDs)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.PromoCode])
	return out, MapError(err)
}

func (r *promoRepository) Update(ctx context.Context, id string, p *db.PromoCode) (*db.PromoCode, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`UPDATE promo_codes SET code = $2, type = $3, value = $4, valid_from = $5, valid_until = $6,
		 usage_limit = $7, plan_ids = $8, updated_at = now() WHERE id = $1 RETURNING `+promoColumns,
		id, p.Code, p.Type, p.Value, p.ValidFrom, p.ValidUntil, p.UsageLimit, p.PlanIDs)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.PromoCode])
	return out, MapError(err)
}

func (r *promoRepository) Delete(ctx context.Context, id string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	tag, err := r.store.Conn(ctx).Exec(ctx, `DELETE FROM promo_codes WHERE id = $1`, id)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *promoRepository) IncrementUsage(ctx context.Context, id string) (bool, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	tag, err := r.store.Conn(ctx).Exec(ctx,
		`UPDATE promo_codes SET used_count = used_count + 1, updated_at = now()
		 WHERE id = $1 AND (usage_limit = 0 OR used_count < usage_limit)`, id)
	if err != nil {
		return false, MapError(err)
	}
	return tag.RowsAffected() == 1, nil
}

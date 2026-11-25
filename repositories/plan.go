package repositories

import (
	"context"

	"9itchi-payments-api/models/db"

	"github.com/jackc/pgx/v5"
)

const planColumns = `id, name, price, regular_price, launch_ends_on, duration_days, quotas, details, active, created_at, updated_at`

// PlanRepository is the only layer that touches PostgreSQL for plan data.
type PlanRepository interface {
	// List returns plans ordered by price; onlyActive filters out deactivated ones.
	List(ctx context.Context, onlyActive bool) ([]db.Plan, error)
	GetByID(ctx context.Context, id string) (*db.Plan, error)
	GetByName(ctx context.Context, name string) (*db.Plan, error)
	Create(ctx context.Context, p *db.Plan) (*db.Plan, error)
	Update(ctx context.Context, id string, p *db.Plan) (*db.Plan, error)
	// Deactivate sets active=false (soft delete).
	Deactivate(ctx context.Context, id string) (*db.Plan, error)
}

type planRepository struct {
	store *Store
}

func NewPlanRepository(store *Store) PlanRepository {
	return &planRepository{store: store}
}

func (r *planRepository) List(ctx context.Context, onlyActive bool) ([]db.Plan, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT `+planColumns+` FROM plans WHERE ($1 = false OR active) ORDER BY price, name`, onlyActive)
	if err != nil {
		return nil, MapError(err)
	}
	plans, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.Plan])
	return plans, MapError(err)
}

func (r *planRepository) GetByID(ctx context.Context, id string) (*db.Plan, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx, `SELECT `+planColumns+` FROM plans WHERE id = $1`, id)
	if err != nil {
		return nil, MapError(err)
	}
	p, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Plan])
	return p, MapError(err)
}

func (r *planRepository) GetByName(ctx context.Context, name string) (*db.Plan, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx, `SELECT `+planColumns+` FROM plans WHERE name = $1`, name)
	if err != nil {
		return nil, MapError(err)
	}
	p, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Plan])
	return p, MapError(err)
}

func (r *planRepository) Create(ctx context.Context, p *db.Plan) (*db.Plan, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`INSERT INTO plans (name, price, regular_price, launch_ends_on, duration_days, quotas, details, active) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING `+planColumns, p.Name, p.Price, p.RegularPrice, p.LaunchEndsOn, p.DurationDays, p.Quotas, detailsOrEmpty(p.Details), p.Active)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Plan])
	return out, MapError(err)
}

func (r *planRepository) Update(ctx context.Context, id string, p *db.Plan) (*db.Plan, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`UPDATE plans SET name = $2, price = $3, regular_price = $4, launch_ends_on = $5, duration_days = $6, quotas = $7, details = $8, active = $9, updated_at = now()
		 WHERE id = $1 RETURNING `+planColumns, id, p.Name, p.Price, p.RegularPrice, p.LaunchEndsOn, p.DurationDays, p.Quotas, detailsOrEmpty(p.Details), p.Active)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Plan])
	return out, MapError(err)
}

func (r *planRepository) Deactivate(ctx context.Context, id string) (*db.Plan, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`UPDATE plans SET active = false, updated_at = now() WHERE id = $1 RETURNING `+planColumns, id)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Plan])
	return out, MapError(err)
}

// detailsOrEmpty keeps a nil map from being written as NULL: the column is never null, {} means no text.
func detailsOrEmpty(d db.PlanDetails) db.PlanDetails {
	if d == nil {
		return db.PlanDetails{}
	}
	return d
}

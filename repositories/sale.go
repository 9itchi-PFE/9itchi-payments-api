package repositories

import (
	"context"
	"time"

	"9itchi-payments-api/models/db"

	"github.com/jackc/pgx/v5"
)

const saleColumns = `id, name, percent, starts_on, ends_on, plan_ids, created_at, updated_at`

// SaleRepository is the only layer that touches PostgreSQL for the sales.
type SaleRepository interface {
	// List returns every sale, the latest to end first.
	List(ctx context.Context) ([]db.Sale, error)
	GetByID(ctx context.Context, id string) (*db.Sale, error)
	// RunningOn returns the sales that include that day.
	RunningOn(ctx context.Context, day time.Time) ([]db.Sale, error)
	Create(ctx context.Context, s *db.Sale) (*db.Sale, error)
	Update(ctx context.Context, id string, s *db.Sale) (*db.Sale, error)
	Delete(ctx context.Context, id string) error
}

type saleRepository struct {
	store *Store
}

func NewSaleRepository(store *Store) SaleRepository {
	return &saleRepository{store: store}
}

func (r *saleRepository) List(ctx context.Context) ([]db.Sale, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx, `SELECT `+saleColumns+` FROM sales ORDER BY ends_on DESC, starts_on DESC, name LIMIT 200`)
	if err != nil {
		return nil, MapError(err)
	}
	sales, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.Sale])
	return sales, MapError(err)
}

func (r *saleRepository) GetByID(ctx context.Context, id string) (*db.Sale, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx, `SELECT `+saleColumns+` FROM sales WHERE id = $1`, id)
	if err != nil {
		return nil, MapError(err)
	}
	s, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Sale])
	return s, MapError(err)
}

func (r *saleRepository) RunningOn(ctx context.Context, day time.Time) ([]db.Sale, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`SELECT `+saleColumns+` FROM sales WHERE starts_on <= $1::date AND ends_on >= $1::date ORDER BY percent DESC, ends_on`,
		day.Format(time.DateOnly))
	if err != nil {
		return nil, MapError(err)
	}
	sales, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.Sale])
	return sales, MapError(err)
}

// planIDs is the plans of a sale as the column wants them: an empty list, never NULL, when it is for every plan.
func planIDs(s *db.Sale) []string {
	if s.PlanIDs == nil {
		return []string{}
	}
	return s.PlanIDs
}

func (r *saleRepository) Create(ctx context.Context, s *db.Sale) (*db.Sale, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`INSERT INTO sales (name, percent, starts_on, ends_on, plan_ids) VALUES ($1, $2, $3, $4, $5) RETURNING `+saleColumns,
		s.Name, s.Percent, s.StartsOn, s.EndsOn, planIDs(s))
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Sale])
	return out, MapError(err)
}

func (r *saleRepository) Update(ctx context.Context, id string, s *db.Sale) (*db.Sale, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`UPDATE sales SET name = $2, percent = $3, starts_on = $4, ends_on = $5, plan_ids = $6, updated_at = now()
		 WHERE id = $1 RETURNING `+saleColumns,
		id, s.Name, s.Percent, s.StartsOn, s.EndsOn, planIDs(s))
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Sale])
	return out, MapError(err)
}

func (r *saleRepository) Delete(ctx context.Context, id string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	tag, err := r.store.Conn(ctx).Exec(ctx, `DELETE FROM sales WHERE id = $1`, id)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

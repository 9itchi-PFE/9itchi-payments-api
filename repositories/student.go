package repositories

import (
	"context"

	"9itchi-payments-api/models/db"

	"github.com/jackc/pgx/v5"
)

const studentColumns = `percent, domains, updated_by, updated_at`

// StudentDiscountRepository is the only layer that touches PostgreSQL for the student discount.
type StudentDiscountRepository interface {
	// Get returns the single student discount row (seeded off by the migrations).
	Get(ctx context.Context) (*db.StudentDiscount, error)
	// Update replaces the percentage and the domains and records who changed them.
	Update(ctx context.Context, userID string, d *db.StudentDiscount) (*db.StudentDiscount, error)
}

type studentDiscountRepository struct {
	store *Store
}

func NewStudentDiscountRepository(store *Store) StudentDiscountRepository {
	return &studentDiscountRepository{store: store}
}

func (r *studentDiscountRepository) Get(ctx context.Context) (*db.StudentDiscount, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx, `SELECT `+studentColumns+` FROM student_discount WHERE id`)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.StudentDiscount])
	return out, MapError(err)
}

func (r *studentDiscountRepository) Update(ctx context.Context, userID string, d *db.StudentDiscount) (*db.StudentDiscount, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	domains := d.Domains
	if domains == nil {
		domains = []string{}
	}
	rows, err := r.store.Conn(ctx).Query(ctx,
		`UPDATE student_discount SET percent = $1, domains = $2, updated_by = $3, updated_at = now()
		 WHERE id RETURNING `+studentColumns, d.Percent, domains, userID)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.StudentDiscount])
	return out, MapError(err)
}

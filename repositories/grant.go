package repositories

import (
	"context"

	"9itchi-payments-api/models/db"

	"github.com/jackc/pgx/v5"
)

const grantColumns = `email, quotas, note, given_by, created_at, updated_at`

// FreeGrantRepository is the only layer that touches PostgreSQL for the free usage given to emails.
// Emails are stored lower-cased; callers pass them lower-cased.
type FreeGrantRepository interface {
	// List returns every grant, the last changed first.
	List(ctx context.Context) ([]db.FreeGrant, error)
	// Get returns the grant of one email (ErrNotFound when none).
	Get(ctx context.Context, email string) (*db.FreeGrant, error)
	// Upsert gives the same quotas and note to every email, overwriting a grant that exists.
	Upsert(ctx context.Context, adminID string, emails []string, quotas map[string]int, note string) ([]db.FreeGrant, error)
	// Update replaces the quotas and note of one email (ErrNotFound when none).
	Update(ctx context.Context, adminID, email string, quotas map[string]int, note string) (*db.FreeGrant, error)
	Delete(ctx context.Context, email string) error
}

type freeGrantRepository struct {
	store *Store
}

func NewFreeGrantRepository(store *Store) FreeGrantRepository {
	return &freeGrantRepository{store: store}
}

func (r *freeGrantRepository) List(ctx context.Context) ([]db.FreeGrant, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx, `SELECT `+grantColumns+` FROM free_grants ORDER BY updated_at DESC, email`)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.FreeGrant])
	return out, MapError(err)
}

func (r *freeGrantRepository) Get(ctx context.Context, email string) (*db.FreeGrant, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx, `SELECT `+grantColumns+` FROM free_grants WHERE email = $1`, email)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.FreeGrant])
	return out, MapError(err)
}

func (r *freeGrantRepository) Upsert(ctx context.Context, adminID string, emails []string, quotas map[string]int, note string) ([]db.FreeGrant, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`INSERT INTO free_grants (email, quotas, note, given_by)
		 SELECT e, $2, $3, $4 FROM unnest($1::text[]) AS e
		 ON CONFLICT (email) DO UPDATE SET quotas = EXCLUDED.quotas, note = EXCLUDED.note,
		   given_by = EXCLUDED.given_by, updated_at = now()
		 RETURNING `+grantColumns, emails, quotas, note, nullUUID(adminID))
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.FreeGrant])
	return out, MapError(err)
}

func (r *freeGrantRepository) Update(ctx context.Context, adminID, email string, quotas map[string]int, note string) (*db.FreeGrant, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`UPDATE free_grants SET quotas = $2, note = $3, given_by = $4, updated_at = now() WHERE email = $1
		 RETURNING `+grantColumns, email, quotas, note, nullUUID(adminID))
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.FreeGrant])
	return out, MapError(err)
}

func (r *freeGrantRepository) Delete(ctx context.Context, email string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	tag, err := r.store.Conn(ctx).Exec(ctx, `DELETE FROM free_grants WHERE email = $1`, email)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// nullUUID is nil for an empty id, so a missing admin is stored as NULL rather than refused.
func nullUUID(id string) *string {
	if id == "" {
		return nil
	}
	return &id
}

package repositories

import (
	"context"

	"9itchi-payments-api/models/db"

	"github.com/jackc/pgx/v5"
)

const issuerColumns = `legal_name, address, tax_id, vat_mention, updated_by, updated_at`

// IssuerRepository is the only layer that touches PostgreSQL for the receipt issuer.
type IssuerRepository interface {
	// Get returns the single issuer row (seeded empty by the migrations).
	Get(ctx context.Context) (*db.ReceiptIssuer, error)
	// Update replaces the four fields and records who changed them.
	Update(ctx context.Context, userID string, i *db.ReceiptIssuer) (*db.ReceiptIssuer, error)
}

type issuerRepository struct {
	store *Store
}

func NewIssuerRepository(store *Store) IssuerRepository {
	return &issuerRepository{store: store}
}

func (r *issuerRepository) Get(ctx context.Context) (*db.ReceiptIssuer, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx, `SELECT `+issuerColumns+` FROM receipt_issuer WHERE id`)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.ReceiptIssuer])
	return out, MapError(err)
}

func (r *issuerRepository) Update(ctx context.Context, userID string, i *db.ReceiptIssuer) (*db.ReceiptIssuer, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx,
		`UPDATE receipt_issuer SET legal_name = $1, address = $2, tax_id = $3, vat_mention = $4, updated_by = $5, updated_at = now()
		 WHERE id RETURNING `+issuerColumns, i.LegalName, i.Address, i.TaxID, i.VATMention, userID)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.ReceiptIssuer])
	return out, MapError(err)
}

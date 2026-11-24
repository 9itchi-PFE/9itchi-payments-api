package db

import "time"

// ReceiptIssuer maps the single row of table receipt_issuer: who issues the receipts.
type ReceiptIssuer struct {
	LegalName  string    `db:"legal_name" json:"legal_name"`
	Address    string    `db:"address" json:"address"`
	TaxID      string    `db:"tax_id" json:"tax_id"`
	VATMention string    `db:"vat_mention" json:"vat_mention"`
	UpdatedBy  *string   `db:"updated_by" json:"updated_by"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

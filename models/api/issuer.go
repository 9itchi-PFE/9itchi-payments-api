package api

import "time"

// UpdateReceiptIssuerRequest is the validated body of UpdateReceiptIssuer (full replacement).
// Every field may be empty: the receipt then shows a blank for it.
type UpdateReceiptIssuerRequest struct {
	LegalName  string `json:"legal_name" binding:"max=200"`
	Address    string `json:"address" binding:"max=500"`
	TaxID      string `json:"tax_id" binding:"max=50"`
	VATMention string `json:"vat_mention" binding:"max=200"`
}

// ReceiptIssuerResponse is the legal information printed on receipts. An empty field is not known yet.
type ReceiptIssuerResponse struct {
	LegalName  string    `json:"legal_name"`
	Address    string    `json:"address"`
	TaxID      string    `json:"tax_id"`
	VATMention string    `json:"vat_mention"`
	Complete   bool      `json:"complete"`
	UpdatedAt  time.Time `json:"updated_at"`
}

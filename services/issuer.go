package services

import (
	"context"
	"strings"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/repositories"
)

// IssuerService holds the business logic for the legal information printed on receipts.
type IssuerService interface {
	// GET /receipt-issuer: what a receipt prints about who issued it
	GetReceiptIssuer(ctx context.Context) (*api.ReceiptIssuerResponse, *models.ApiError)
	// PUT /admin/receipt-issuer: replace the legal information (empty fields allowed)
	UpdateReceiptIssuer(ctx context.Context, userID string, req api.UpdateReceiptIssuerRequest) (*api.ReceiptIssuerResponse, *models.ApiError)
}

type issuerService struct {
	repo repositories.IssuerRepository
}

func NewIssuerService(repo repositories.IssuerRepository) IssuerService {
	return &issuerService{repo: repo}
}

func issuerResponse(i *db.ReceiptIssuer) *api.ReceiptIssuerResponse {
	return &api.ReceiptIssuerResponse{
		LegalName: i.LegalName, Address: i.Address, TaxID: i.TaxID, VATMention: i.VATMention,
		Complete:  i.LegalName != "" && i.Address != "" && i.TaxID != "" && i.VATMention != "",
		UpdatedAt: i.UpdatedAt,
	}
}

func (s *issuerService) GetReceiptIssuer(ctx context.Context) (*api.ReceiptIssuerResponse, *models.ApiError) {
	i, err := s.repo.Get(ctx)
	if err != nil {
		return nil, repoError(err, "receipt issuer not found")
	}
	return issuerResponse(i), nil
}

func (s *issuerService) UpdateReceiptIssuer(ctx context.Context, userID string, req api.UpdateReceiptIssuerRequest) (*api.ReceiptIssuerResponse, *models.ApiError) {
	if !isUUID(userID) {
		return nil, badRequest("invalid user id")
	}
	in := &db.ReceiptIssuer{
		LegalName:  strings.TrimSpace(req.LegalName),
		Address:    strings.TrimSpace(req.Address),
		TaxID:      strings.TrimSpace(req.TaxID),
		VATMention: strings.TrimSpace(req.VATMention),
	}
	out, err := s.repo.Update(ctx, userID, in)
	if err != nil {
		return nil, repoError(err, "receipt issuer not found")
	}
	return issuerResponse(out), nil
}

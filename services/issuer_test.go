package services

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/repositories"
	"9itchi-payments-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetReceiptIssuer(t *testing.T) {
	t.Run("nothing filled in yet", func(t *testing.T) {
		res, apiErr := NewIssuerService(&mocks.IssuerRepository{}).GetReceiptIssuer(context.Background())
		require.Nil(t, apiErr)
		assert.Empty(t, res.LegalName)
		assert.False(t, res.Complete)
	})
	t.Run("one field missing is not complete", func(t *testing.T) {
		repo := &mocks.IssuerRepository{GetFn: func(context.Context) (*db.ReceiptIssuer, error) {
			return &db.ReceiptIssuer{LegalName: "Example SARL", Address: "1 rue Exemple, Tunis", TaxID: "0000000/A/A/M/000"}, nil
		}}
		res, apiErr := NewIssuerService(repo).GetReceiptIssuer(context.Background())
		require.Nil(t, apiErr)
		assert.Equal(t, "Example SARL", res.LegalName)
		assert.False(t, res.Complete)
	})
	t.Run("all four fields is complete", func(t *testing.T) {
		repo := &mocks.IssuerRepository{GetFn: func(context.Context) (*db.ReceiptIssuer, error) {
			return &db.ReceiptIssuer{LegalName: "Example SARL", Address: "1 rue Exemple, Tunis", TaxID: "0000000/A/A/M/000", VATMention: "TVA incluse"}, nil
		}}
		res, apiErr := NewIssuerService(repo).GetReceiptIssuer(context.Background())
		require.Nil(t, apiErr)
		assert.True(t, res.Complete)
	})
	t.Run("repository error", func(t *testing.T) {
		repo := &mocks.IssuerRepository{GetFn: func(context.Context) (*db.ReceiptIssuer, error) { return nil, errors.New("x") }}
		_, apiErr := NewIssuerService(repo).GetReceiptIssuer(context.Background())
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	})
	t.Run("missing row", func(t *testing.T) {
		repo := &mocks.IssuerRepository{GetFn: func(context.Context) (*db.ReceiptIssuer, error) { return nil, repositories.ErrNotFound }}
		_, apiErr := NewIssuerService(repo).GetReceiptIssuer(context.Background())
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
	})
}

func TestUpdateReceiptIssuer(t *testing.T) {
	t.Run("trims and records who changed it", func(t *testing.T) {
		var by string
		var saved *db.ReceiptIssuer
		repo := &mocks.IssuerRepository{UpdateFn: func(_ context.Context, uid string, i *db.ReceiptIssuer) (*db.ReceiptIssuer, error) {
			by, saved = uid, i
			return i, nil
		}}
		res, apiErr := NewIssuerService(repo).UpdateReceiptIssuer(context.Background(), userID,
			api.UpdateReceiptIssuerRequest{LegalName: "  Example SARL ", Address: strings.Repeat(" ", 3), TaxID: "0000000/A/A/M/000"})
		require.Nil(t, apiErr)
		assert.Equal(t, userID, by)
		assert.Equal(t, "Example SARL", saved.LegalName)
		assert.Empty(t, saved.Address, "a blank field stays empty: the receipt shows a gap for it")
		assert.False(t, res.Complete)
	})
	t.Run("invalid caller id", func(t *testing.T) {
		_, apiErr := NewIssuerService(&mocks.IssuerRepository{}).UpdateReceiptIssuer(context.Background(), "nope", api.UpdateReceiptIssuerRequest{})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})
	t.Run("repository error", func(t *testing.T) {
		repo := &mocks.IssuerRepository{UpdateFn: func(context.Context, string, *db.ReceiptIssuer) (*db.ReceiptIssuer, error) {
			return nil, errors.New("x")
		}}
		_, apiErr := NewIssuerService(repo).UpdateReceiptIssuer(context.Background(), userID, api.UpdateReceiptIssuerRequest{})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	})
}

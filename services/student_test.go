package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"9itchi-payments-api/core"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func students(percent int, domains ...string) *mocks.StudentDiscountRepository {
	return &mocks.StudentDiscountRepository{GetFn: func(context.Context) (*db.StudentDiscount, error) {
		return &db.StudentDiscount{Percent: percent, Domains: domains}, nil
	}}
}

func TestStudentDomain(t *testing.T) {
	domains := []string{"enis.tn", "etudiant-isi.utm.tn"}
	for email, want := range map[string]string{
		"ali@enis.tn":             "enis.tn",
		" Ali@ENIS.tn ":           "enis.tn",
		"ali@info.enis.tn":        "enis.tn",
		"ali@etudiant-isi.utm.tn": "etudiant-isi.utm.tn",
		"ali@utm.tn":              "",
		"ali@notenis.tn":          "",
		"ali@enis.tn.evil.com":    "",
		"ali@gmail.com":           "",
		"no-at-sign":              "",
		"":                        "",
	} {
		assert.Equal(t, want, studentDomain(email, domains), email)
	}
}

func TestUpdateStudentDiscount(t *testing.T) {
	ctx := context.Background()
	pct := func(n int) *int { return &n }

	t.Run("normalizes and dedupes domains", func(t *testing.T) {
		var saved *db.StudentDiscount
		repo := &mocks.StudentDiscountRepository{UpdateFn: func(_ context.Context, by string, d *db.StudentDiscount) (*db.StudentDiscount, error) {
			assert.Equal(t, userID, by)
			saved = d
			return d, nil
		}}
		res, apiErr := NewStudentDiscountService(repo).UpdateStudentDiscount(ctx, userID,
			api.UpdateStudentDiscountRequest{Percent: pct(30), Domains: []string{" @ENIS.tn", ".enis.tn", "etudiant-isi.utm.tn", ""}})
		require.Nil(t, apiErr)
		assert.Equal(t, []string{"enis.tn", "etudiant-isi.utm.tn"}, saved.Domains)
		assert.Equal(t, 30, res.Percent)
	})
	t.Run("refuses bad input", func(t *testing.T) {
		svc := NewStudentDiscountService(&mocks.StudentDiscountRepository{})
		for name, req := range map[string]api.UpdateStudentDiscountRequest{
			"no percent":   {Domains: []string{"enis.tn"}},
			"over 100":     {Percent: pct(101)},
			"negative":     {Percent: pct(-1)},
			"no dot":       {Percent: pct(10), Domains: []string{"localhost"}},
			"email given":  {Percent: pct(10), Domains: []string{"ali@enis.tn"}},
			"space inside": {Percent: pct(10), Domains: []string{"en is.tn"}},
		} {
			_, apiErr := svc.UpdateStudentDiscount(ctx, userID, req)
			require.NotNil(t, apiErr, name)
			assert.Equal(t, http.StatusBadRequest, apiErr.Code, name)
		}
		_, apiErr := svc.UpdateStudentDiscount(ctx, "x", api.UpdateStudentDiscountRequest{Percent: pct(10)})
		require.NotNil(t, apiErr)
	})
	t.Run("repo failure", func(t *testing.T) {
		repo := &mocks.StudentDiscountRepository{UpdateFn: func(context.Context, string, *db.StudentDiscount) (*db.StudentDiscount, error) {
			return nil, errors.New("boom")
		}}
		_, apiErr := NewStudentDiscountService(repo).UpdateStudentDiscount(ctx, userID, api.UpdateStudentDiscountRequest{Percent: pct(10)})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	})
}

func TestMyStudentDiscount(t *testing.T) {
	ctx := context.Background()
	svc := NewStudentDiscountService(students(25, "enis.tn"))
	for name, tc := range map[string]struct {
		email    string
		verified bool
		want     api.MyStudentDiscountResponse
	}{
		"verified student":   {"ali@enis.tn", true, api.MyStudentDiscountResponse{Eligible: true, Percent: 25, Domain: "enis.tn"}},
		"unverified student": {"ali@enis.tn", false, api.MyStudentDiscountResponse{Percent: 25, Domain: "enis.tn", NeedsVerifiedEmail: true}},
		"other domain":       {"ali@gmail.com", true, api.MyStudentDiscountResponse{}},
	} {
		res, apiErr := svc.MyStudentDiscount(ctx, tc.email, tc.verified)
		require.Nil(t, apiErr, name)
		assert.Equal(t, tc.want, *res, name)
	}
	off, apiErr := NewStudentDiscountService(students(0, "enis.tn")).MyStudentDiscount(ctx, "ali@enis.tn", true)
	require.Nil(t, apiErr)
	assert.Equal(t, api.MyStudentDiscountResponse{}, *off, "0% means switched off")
}

func TestCheckoutStudentDiscount(t *testing.T) {
	ctx := context.Background()
	save := &mocks.PromoRepository{GetByCodeFn: func(context.Context, string) (*db.PromoCode, error) {
		return &db.PromoCode{ID: "p1", Code: "SAVE", Type: "percentage", Value: 20,
			ValidFrom: fixedNow.Add(-1e12), ValidUntil: fixedNow.Add(1e12)}, nil
	}}
	run := func(t *testing.T, req api.CheckoutRequest, wantAmount, wantDiscount int64, wantPercent int) {
		t.Helper()
		var created *db.Payment
		pays := &mocks.PaymentRepository{CreateFn: func(_ context.Context, p *db.Payment) (*db.Payment, error) {
			c := *p
			c.ID = payID
			created = &c
			return &c, nil
		}}
		prov := &mocks.PaymentProvider{CreateCheckoutFn: func(_ context.Context, r core.CheckoutRequest) (core.CheckoutResult, error) {
			assert.EqualValues(t, wantAmount, r.AmountMillimes)
			return core.CheckoutResult{Ref: "ref-1", URL: "https://pay.example/ref-1"}, nil
		}}
		svc := newSubSvc(&mocks.SubscriptionRepository{}, activePlan(50), save, pays, prov)
		svc.students = students(10, "enis.tn")
		req.PlanID = planA
		_, apiErr := svc.Checkout(ctx, userID, req)
		require.Nil(t, apiErr)
		require.NotNil(t, created)
		assert.EqualValues(t, wantAmount, created.AmountMillimes)
		assert.EqualValues(t, wantDiscount, created.DiscountMillimes)
		assert.Equal(t, wantPercent, created.StudentPercent)
	}
	t.Run("verified student", func(t *testing.T) {
		run(t, api.CheckoutRequest{Email: "ali@enis.tn", EmailVerified: true}, 45000, 5000, 10)
	})
	t.Run("stacks on a promo code", func(t *testing.T) {
		// 50 TND - 20% = 40 TND, then - 10% = 36 TND.
		run(t, api.CheckoutRequest{Email: "ali@enis.tn", EmailVerified: true, PromoCode: "SAVE"}, 36000, 14000, 10)
	})
	t.Run("unverified email gets nothing", func(t *testing.T) {
		run(t, api.CheckoutRequest{Email: "ali@enis.tn"}, 50000, 0, 0)
	})
	t.Run("other domain gets nothing", func(t *testing.T) {
		run(t, api.CheckoutRequest{Email: "ali@gmail.com", EmailVerified: true}, 50000, 0, 0)
	})
	t.Run("read failure stops checkout", func(t *testing.T) {
		svc := newSubSvc(&mocks.SubscriptionRepository{}, activePlan(50), save, &mocks.PaymentRepository{}, &mocks.PaymentProvider{})
		svc.students = &mocks.StudentDiscountRepository{GetFn: func(context.Context) (*db.StudentDiscount, error) { return nil, errors.New("boom") }}
		_, apiErr := svc.Checkout(ctx, userID, api.CheckoutRequest{PlanID: planA, Email: "ali@enis.tn", EmailVerified: true})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	})
}

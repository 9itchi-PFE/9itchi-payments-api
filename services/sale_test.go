package services

import (
	"context"
	"net/http"
	"testing"
	"time"

	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/repositories"
	"9itchi-payments-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	paidPlan = "44444444-4444-4444-8444-444444444444"
	freeID   = "55555555-5555-4555-8555-555555555555"
)

func newSaleSvc(sales *mocks.SaleRepository) *saleService {
	plans := &mocks.PlanRepository{GetByIDFn: func(_ context.Context, id string) (*db.Plan, error) {
		switch id {
		case paidPlan:
			return &db.Plan{ID: id, Name: "Mensuel", Price: 50}, nil
		case freeID:
			return &db.Plan{ID: id, Name: FreePlanName}, nil
		}
		return nil, repositories.ErrNotFound
	}}
	svc := NewSaleService(sales, plans).(*saleService)
	svc.now = func() time.Time { return time.Date(2027, 2, 10, 9, 0, 0, 0, time.UTC) }
	return svc
}

func TestCreateSale(t *testing.T) {
	repo := &mocks.SaleRepository{}
	svc := newSaleSvc(repo)
	res, apiErr := svc.CreateSale(context.Background(), api.SaleRequest{
		Name: "  Rentrée ", Percent: 30, StartsOn: "2027-02-15", EndsOn: "2027-02-28", PlanIDs: []string{paidPlan, paidPlan},
	})
	require.Nil(t, apiErr)
	assert.Equal(t, "Rentrée", res.Name)
	assert.Equal(t, "upcoming", res.State)
	assert.Equal(t, []string{paidPlan}, res.PlanIDs)
	assert.Equal(t, "2027-02-28", res.EndsOn)

	// One day, today, for every plan.
	res, apiErr = svc.CreateSale(context.Background(), api.SaleRequest{Name: "Flash", Percent: 90, StartsOn: "2027-02-10", EndsOn: "2027-02-10"})
	require.Nil(t, apiErr)
	assert.Equal(t, "running", res.State)
	assert.Empty(t, res.PlanIDs)

	for name, req := range map[string]api.SaleRequest{
		"no name":        {Name: " ", Percent: 30, StartsOn: "2027-02-15", EndsOn: "2027-02-28"},
		"nothing off":    {Name: "x", Percent: 0, StartsOn: "2027-02-15", EndsOn: "2027-02-28"},
		"everything off": {Name: "x", Percent: 100, StartsOn: "2027-02-15", EndsOn: "2027-02-28"},
		"not a day":      {Name: "x", Percent: 30, StartsOn: "15/02/2027", EndsOn: "2027-02-28"},
		"ends first":     {Name: "x", Percent: 30, StartsOn: "2027-02-28", EndsOn: "2027-02-15"},
		"already over":   {Name: "x", Percent: 30, StartsOn: "2027-01-01", EndsOn: "2027-02-09"},
		"bad plan id":    {Name: "x", Percent: 30, StartsOn: "2027-02-15", EndsOn: "2027-02-28", PlanIDs: []string{"nope"}},
		"the free plan":  {Name: "x", Percent: 30, StartsOn: "2027-02-15", EndsOn: "2027-02-28", PlanIDs: []string{freeID}},
	} {
		t.Run(name, func(t *testing.T) {
			_, apiErr := svc.CreateSale(context.Background(), req)
			require.NotNil(t, apiErr)
			assert.Equal(t, http.StatusBadRequest, apiErr.Code)
		})
	}
	_, apiErr = svc.CreateSale(context.Background(), api.SaleRequest{Name: "x", Percent: 30, StartsOn: "2027-02-15", EndsOn: "2027-02-28",
		PlanIDs: []string{"66666666-6666-4666-8666-666666666666"}})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)
	assert.Len(t, repo.Sales, 2, "a refused sale is not stored")

	repo.Err = assert.AnError
	_, apiErr = svc.CreateSale(context.Background(), api.SaleRequest{Name: "x", Percent: 30, StartsOn: "2027-02-15", EndsOn: "2027-02-28"})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
}

func TestListUpdateDeleteSale(t *testing.T) {
	d := func(m time.Month, day int) time.Time { return time.Date(2027, m, day, 0, 0, 0, 0, time.UTC) }
	repo := &mocks.SaleRepository{Sales: []db.Sale{
		{ID: "11111111-1111-4111-8111-111111111111", Name: "Passée", Percent: 10, StartsOn: d(1, 1), EndsOn: d(1, 10)},
		{ID: "22222222-2222-4222-8222-222222222222", Name: "Plus tard", Percent: 20, StartsOn: d(3, 1), EndsOn: d(3, 10)},
		{ID: "33333333-3333-4333-8333-333333333333", Name: "En cours", Percent: 30, StartsOn: d(2, 1), EndsOn: d(2, 15)},
		{ID: "77777777-7777-4777-8777-777777777777", Name: "Bientôt", Percent: 15, StartsOn: d(2, 20), EndsOn: d(2, 25)},
	}}
	svc := newSaleSvc(repo)
	list, apiErr := svc.ListSales(context.Background())
	require.Nil(t, apiErr)
	var names, states []string
	for _, s := range list.Items {
		names, states = append(names, s.Name), append(states, s.State)
	}
	assert.Equal(t, []string{"En cours", "Bientôt", "Plus tard", "Passée"}, names)
	assert.Equal(t, []string{"running", "upcoming", "upcoming", "over"}, states)

	// Ending a running sale earlier (today is its last day), or changing what it takes off.
	req := api.SaleRequest{Name: "En cours", Percent: 25, StartsOn: "2027-02-01", EndsOn: "2027-02-10"}
	res, apiErr := svc.UpdateSale(context.Background(), "33333333-3333-4333-8333-333333333333", req)
	require.Nil(t, apiErr)
	assert.Equal(t, 25, res.Percent)
	assert.Equal(t, "running", res.State)
	_, apiErr = svc.UpdateSale(context.Background(), "nope", req)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	_, apiErr = svc.UpdateSale(context.Background(), "99999999-9999-4999-8999-999999999999", req)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)

	require.Nil(t, svc.DeleteSale(context.Background(), "33333333-3333-4333-8333-333333333333"))
	assert.Len(t, repo.Sales, 3)
	assert.Equal(t, http.StatusNotFound, svc.DeleteSale(context.Background(), "33333333-3333-4333-8333-333333333333").Code)
	assert.Equal(t, http.StatusBadRequest, svc.DeleteSale(context.Background(), "nope").Code)

	repo.Err = assert.AnError
	_, apiErr = svc.ListSales(context.Background())
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
}

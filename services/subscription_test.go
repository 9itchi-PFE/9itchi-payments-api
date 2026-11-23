package services

import (
	"context"
	"errors"
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

func TestMySubscription(t *testing.T) {
	plans := &mocks.PlanRepository{
		GetByIDFn:   func(context.Context, string) (*db.Plan, error) { return &db.Plan{ID: planA, Name: "Pro"}, nil },
		GetByNameFn: func(context.Context, string) (*db.Plan, error) { return freePlan, nil },
	}
	t.Run("active subscription with plan", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: func(_ context.Context, uid string) (*db.Subscription, error) {
			assert.Equal(t, userID, uid)
			return &db.Subscription{ID: "s", PlanID: planA, Status: "active", EndsAt: time.Now().Add(time.Hour)}, nil
		}}
		res, apiErr := NewSubscriptionService(subs, plans).MySubscription(context.Background(), userID)
		require.Nil(t, apiErr)
		require.NotNil(t, res.Subscription)
		assert.Equal(t, "s", res.Subscription.ID)
		assert.Equal(t, "Pro", res.Plan.Name)
		assert.False(t, res.IsFree)
	})
	t.Run("free fallback", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) {
			return nil, repositories.ErrNotFound
		}}
		res, apiErr := NewSubscriptionService(subs, plans).MySubscription(context.Background(), userID)
		require.Nil(t, apiErr)
		assert.Nil(t, res.Subscription)
		assert.True(t, res.IsFree)
		assert.Equal(t, FreePlanName, res.Plan.Name)
	})
	t.Run("repo failure", func(t *testing.T) {
		subs := &mocks.SubscriptionRepository{GetActiveByUserFn: func(context.Context, string) (*db.Subscription, error) {
			return nil, errors.New("db down")
		}}
		_, apiErr := NewSubscriptionService(subs, plans).MySubscription(context.Background(), userID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	})
	t.Run("bad user id", func(t *testing.T) {
		_, apiErr := NewSubscriptionService(nil, nil).MySubscription(context.Background(), "x")
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})
}

func TestSubscriptionStubsStay501(t *testing.T) {
	svc := NewSubscriptionService(nil, nil)
	_, e1 := svc.Checkout(context.Background(), userID, api.CheckoutRequest{})
	_, e2 := svc.CancelSubscription(context.Background(), userID)
	_, e3 := svc.Webhook(context.Background(), "stripe")
	assert.Equal(t, http.StatusNotImplemented, e1.Code)
	assert.Equal(t, http.StatusNotImplemented, e2.Code)
	assert.Equal(t, http.StatusNotImplemented, e3.Code)
}

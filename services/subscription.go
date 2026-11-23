package services

import (
	"context"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/repositories"
)

// SubscriptionService holds all business logic for subscription. Handlers stay thin.
type SubscriptionService interface {
	// POST /subscriptions/checkout: start payment for a plan (+ promo)
	Checkout(ctx context.Context, userID string, req api.CheckoutRequest) (any, *models.ApiError)
	// GET /subscriptions/me: current subscription
	MySubscription(ctx context.Context, userID string) (any, *models.ApiError)
	// POST /subscriptions/cancel: cancel at period end
	CancelSubscription(ctx context.Context, userID string) (any, *models.ApiError)
	// POST /webhooks/:provider: payment provider callback (signature verified)
	Webhook(ctx context.Context, provider string) (any, *models.ApiError)
}

type subscriptionService struct {
	repo repositories.SubscriptionRepository
}

func NewSubscriptionService(repo repositories.SubscriptionRepository) SubscriptionService {
	return &subscriptionService{repo: repo}
}

func (s *subscriptionService) Checkout(_ context.Context, _ string, _ api.CheckoutRequest) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *subscriptionService) MySubscription(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *subscriptionService) CancelSubscription(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *subscriptionService) Webhook(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

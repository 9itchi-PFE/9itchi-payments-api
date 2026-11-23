package services

import (
	"context"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/models/mapping"
	"9itchi-payments-api/repositories"
)

// SubscriptionService holds all business logic for subscription. Handlers stay thin.
type SubscriptionService interface {
	// POST /subscriptions/checkout: start payment for a plan (+ promo)
	Checkout(ctx context.Context, userID string, req api.CheckoutRequest) (any, *models.ApiError)
	// GET /subscriptions/me: active subscription with its plan, else the Free plan
	MySubscription(ctx context.Context, userID string) (*api.MySubscriptionResponse, *models.ApiError)
	// POST /subscriptions/cancel: cancel at period end
	CancelSubscription(ctx context.Context, userID string) (any, *models.ApiError)
	// POST /webhooks/:provider: payment provider callback (signature verified)
	Webhook(ctx context.Context, provider string) (any, *models.ApiError)
}

type subscriptionService struct {
	repo  repositories.SubscriptionRepository
	plans repositories.PlanRepository
}

func NewSubscriptionService(repo repositories.SubscriptionRepository, plans repositories.PlanRepository) SubscriptionService {
	return &subscriptionService{repo: repo, plans: plans}
}

func (s *subscriptionService) Checkout(_ context.Context, _ string, _ api.CheckoutRequest) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *subscriptionService) MySubscription(ctx context.Context, userID string) (*api.MySubscriptionResponse, *models.ApiError) {
	if !isUUID(userID) {
		return nil, badRequest("invalid user id")
	}
	sub, plan, err := userPlan(ctx, s.repo, s.plans, userID)
	if err != nil {
		return nil, repoError(err, "plan not found")
	}
	resp := &api.MySubscriptionResponse{Plan: mapping.PlanToResponse(plan), IsFree: sub == nil}
	if sub != nil {
		resp.Subscription = mapping.SubscriptionToResponse(sub)
	}
	return resp, nil
}

func (s *subscriptionService) CancelSubscription(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *subscriptionService) Webhook(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

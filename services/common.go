package services

import (
	"context"
	"errors"
	"net/http"
	"regexp"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/db"
	"9itchi-payments-api/repositories"
	"9itchi-payments-api/utils"
)

// FreePlanName is the seeded plan every user falls back to without an active subscription.
const FreePlanName = "Free"

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func isUUID(s string) bool { return uuidRe.MatchString(s) }

func badRequest(msg string) *models.ApiError {
	return &models.ApiError{Code: http.StatusBadRequest, Error: errors.New(msg)}
}

// repoError maps repository errors to API errors; unexpected errors are logged and hidden.
func repoError(err error, notFoundMsg string) *models.ApiError {
	switch {
	case errors.Is(err, repositories.ErrNotFound):
		return &models.ApiError{Code: http.StatusNotFound, Error: errors.New(notFoundMsg)}
	case errors.Is(err, repositories.ErrConflict):
		return &models.ApiError{Code: http.StatusConflict, Error: errors.New("resource already exists")}
	default:
		utils.Logger.Logger.Errorw("repository error", "error", err)
		return &models.ApiError{Code: http.StatusInternalServerError, Error: errors.New("internal server error")}
	}
}

// userPlan returns the user's active subscription (nil when none) and the plan that applies (Free fallback).
func userPlan(ctx context.Context, subs repositories.SubscriptionRepository, plans repositories.PlanRepository, userID string) (*db.Subscription, *db.Plan, error) {
	sub, err := subs.GetActiveByUser(ctx, userID)
	if err == nil {
		plan, err := plans.GetByID(ctx, sub.PlanID)
		return sub, plan, err
	}
	if !errors.Is(err, repositories.ErrNotFound) {
		return nil, nil, err
	}
	plan, err := plans.GetByName(ctx, FreePlanName)
	return nil, plan, err
}

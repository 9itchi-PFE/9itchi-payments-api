package handlers

import (
	"net/http"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/services"

	"github.com/gin-gonic/gin"
)

// Checkout: POST /subscriptions/checkout (user) start payment for a plan (+ promo).
func Checkout(svc services.SubscriptionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.CheckoutRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.Checkout(c.Request.Context(), c.GetString("user_id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// MySubscription: GET /subscriptions/me (user) current subscription.
func MySubscription(svc services.SubscriptionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.MySubscription(c.Request.Context(), c.GetString("user_id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// CancelSubscription: POST /subscriptions/cancel (user) cancel at period end.
func CancelSubscription(svc services.SubscriptionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.CancelSubscription(c.Request.Context(), c.GetString("user_id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// Webhook: POST /webhooks/:provider (public) payment provider callback (signature verified).
func Webhook(svc services.SubscriptionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.Webhook(c.Request.Context(), c.Param("provider"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

package handlers

import (
	"errors"
	"io"
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

// CancelSubscription: POST /subscriptions/cancel (user) stop the renewal; access lasts until the period ends.
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

// maxWebhookBody caps the raw webhook body read into memory.
const maxWebhookBody = 1 << 20

// Webhook: POST /webhooks/:provider (public) payment provider callback (signature verified).
func Webhook(svc services.SubscriptionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookBody))
		if err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("invalid webhook")})
			return
		}
		signature := c.GetHeader("X-Flouci-Signature")
		if signature == "" {
			signature = c.GetHeader("Flouci-Signature")
		}
		result, apiErr := svc.Webhook(c.Request.Context(), c.Param("provider"), body, signature)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// ListPayments: GET /payments (user) completed payments of the user, newest first (receipts).
func ListPayments(svc services.SubscriptionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.ListPayments(c.Request.Context(), c.GetString("user_id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// VerifyPayment: POST /payments/:id/verify (user) where a payment stands, re-checked with the gateway.
func VerifyPayment(svc services.SubscriptionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.VerifyPayment(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// ResumeSubscription: POST /subscriptions/resume (user) renew again a stopped subscription that has not ended.
func ResumeSubscription(svc services.SubscriptionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.ResumeSubscription(c.Request.Context(), c.GetString("user_id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// SendRenewalReminders: POST /internal/subscriptions/renewal-reminders (internal) raise the renewal reminders that are due.
func SendRenewalReminders(svc services.SubscriptionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.SendRenewalReminders(c.Request.Context())
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

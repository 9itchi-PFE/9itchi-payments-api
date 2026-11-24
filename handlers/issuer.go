package handlers

import (
	"net/http"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/services"

	"github.com/gin-gonic/gin"
)

// GetReceiptIssuer: GET /receipt-issuer (user) legal information printed on receipts.
func GetReceiptIssuer(svc services.IssuerService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.GetReceiptIssuer(c.Request.Context())
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// UpdateReceiptIssuer: PUT /admin/receipt-issuer (admin) replace the legal information.
func UpdateReceiptIssuer(svc services.IssuerService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.UpdateReceiptIssuerRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.UpdateReceiptIssuer(c.Request.Context(), c.GetString("user_id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

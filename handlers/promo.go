package handlers

import (
	"net/http"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/services"

	"github.com/gin-gonic/gin"
)

// ValidatePromo: POST /promo-codes/validate (user) check code against a plan.
func ValidatePromo(svc services.PromoService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.ValidatePromoRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.ValidatePromo(c.Request.Context(), c.GetString("user_id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// ListPromos: GET /admin/promo-codes (admin) list codes.
func ListPromos(svc services.PromoService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.ListPromos(c.Request.Context(), c.GetString("user_id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// CreatePromo: POST /admin/promo-codes (admin) percentage or fixed amount.
func CreatePromo(svc services.PromoService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.CreatePromoRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.CreatePromo(c.Request.Context(), c.GetString("user_id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusCreated, result)
	}
}

// UpdatePromo: PUT /admin/promo-codes/:id (admin) edit validity/limits.
func UpdatePromo(svc services.PromoService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.UpdatePromoRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.UpdatePromo(c.Request.Context(), c.GetString("user_id"), c.Param("id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// DeletePromo: DELETE /admin/promo-codes/:id (admin) delete code.
func DeletePromo(svc services.PromoService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.DeletePromo(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

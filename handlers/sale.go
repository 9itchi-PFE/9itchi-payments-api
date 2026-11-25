package handlers

import (
	"net/http"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/services"

	"github.com/gin-gonic/gin"
)

// ListSales: GET /admin/sales (admin) the reductions without a code, running and to come first.
func ListSales(svc services.SaleService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.ListSales(c.Request.Context())
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// CreateSale: POST /admin/sales (admin) a percentage off, between two days.
func CreateSale(svc services.SaleService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.SaleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.CreateSale(c.Request.Context(), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusCreated, result)
	}
}

// UpdateSale: PUT /admin/sales/:id (admin) full replacement.
func UpdateSale(svc services.SaleService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.SaleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.UpdateSale(c.Request.Context(), c.Param("id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// DeleteSale: DELETE /admin/sales/:id (admin) ends a sale at once, or removes one that has not started.
func DeleteSale(svc services.SaleService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiErr := svc.DeleteSale(c.Request.Context(), c.Param("id")); apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

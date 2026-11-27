package handlers

import (
	"net/http"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/services"

	"github.com/gin-gonic/gin"
)

// ListFreeGrants: GET /admin/free-grants (admin) the emails the team gave free usage to.
func ListFreeGrants(svc services.FreeGrantService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.ListGrants(c.Request.Context())
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// GiveFreeGrants: POST /admin/free-grants (admin) the same quotas to one or several emails.
func GiveFreeGrants(svc services.FreeGrantService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.GiveFreeGrantsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.GiveGrants(c.Request.Context(), c.GetString("user_id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// UpdateFreeGrant: PUT /admin/free-grants/:email (admin) replace one email's quotas and note.
func UpdateFreeGrant(svc services.FreeGrantService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.UpdateFreeGrantRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.UpdateGrant(c.Request.Context(), c.GetString("user_id"), c.Param("email"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// DeleteFreeGrant: DELETE /admin/free-grants/:email (admin) take the free usage back.
func DeleteFreeGrant(svc services.FreeGrantService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiErr := svc.DeleteGrant(c.Request.Context(), c.Param("email")); apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

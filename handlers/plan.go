package handlers

import (
	"net/http"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/services"

	"github.com/gin-gonic/gin"
)

// ListPlans: GET /plans (public) active plans.
func ListPlans(svc services.PlanService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.ListPlans(c.Request.Context())
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// CreatePlan: POST /admin/plans (admin) create plan (price, duration, quotas).
func CreatePlan(svc services.PlanService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.CreatePlanRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.CreatePlan(c.Request.Context(), c.GetString("user_id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusCreated, result)
	}
}

// UpdatePlan: PUT /admin/plans/:id (admin) edit plan.
func UpdatePlan(svc services.PlanService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.UpdatePlanRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.UpdatePlan(c.Request.Context(), c.GetString("user_id"), c.Param("id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// DeactivatePlan: DELETE /admin/plans/:id (admin) deactivate plan.
func DeactivatePlan(svc services.PlanService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.DeactivatePlan(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// GetQuotas: GET /internal/quotas/:userId (internal) remaining quotas for a user.
func GetQuotas(svc services.PlanService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.GetQuotas(c.Request.Context(), c.Param("userId"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

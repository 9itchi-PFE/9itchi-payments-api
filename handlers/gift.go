package handlers

import (
	"net/http"

	"9itchi-payments-api/models"
	"9itchi-payments-api/services"

	"github.com/gin-gonic/gin"
)

// GiveMonth: POST /admin/users/:id/gifts (admin) gives the student 30 days; the admin comes from the token.
func GiveMonth(svc services.GiftService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.GiveMonth(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusCreated, result)
	}
}

// ListGifts: GET /admin/users/:id/gifts (admin) what the team gave one student.
func ListGifts(svc services.GiftService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.ListGifts(c.Request.Context(), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

package handlers

import (
	"net/http"

	"9itchi-payments-api/models"
	"9itchi-payments-api/models/api"
	"9itchi-payments-api/services"

	"github.com/gin-gonic/gin"
)

// GetStudentDiscount: GET /admin/student-discount (admin) the percentage and the student domains.
func GetStudentDiscount(svc services.StudentDiscountService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.GetStudentDiscount(c.Request.Context())
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// UpdateStudentDiscount: PUT /admin/student-discount (admin) replace the percentage and the domains.
func UpdateStudentDiscount(svc services.StudentDiscountService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.UpdateStudentDiscountRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.UpdateStudentDiscount(c.Request.Context(), c.GetString("user_id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// MyStudentDiscount: GET /student-discount/me (user) whether the user gets the student discount at checkout.
func MyStudentDiscount(svc services.StudentDiscountService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.MyStudentDiscount(c.Request.Context(), c.GetString("email"), c.GetBool("email_verified"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

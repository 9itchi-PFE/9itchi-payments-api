package router

import (
	"9itchi-payments-api/handlers"
	"9itchi-payments-api/services"

	"github.com/gin-gonic/gin"
)

func InitializeRouter(services *services.Services) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	initializeMiddlewares(r)
	initializeRoutes(r, services)

	return r
}

func initializeRoutes(r *gin.Engine, services *services.Services) {
	// Health endpoints
	r.GET("/", handlers.GetHealth)
	r.GET("/health", handlers.GetHealth)

	// public endpoints
	{
		r.GET("/plans", handlers.ListPlans(services.Plan))
		r.POST("/webhooks/:provider", handlers.Webhook(services.Subscription))
	}

	// user endpoints
	userGroup := r.Group("/")
	userGroup.Use(AuthMiddleware())
	{
		userGroup.POST("/promo-codes/validate", handlers.ValidatePromo(services.Promo))
		userGroup.POST("/subscriptions/checkout", handlers.Checkout(services.Subscription))
		userGroup.GET("/subscriptions/me", handlers.MySubscription(services.Subscription))
		userGroup.POST("/subscriptions/cancel", handlers.CancelSubscription(services.Subscription))
	}

	// admin endpoints
	adminGroup := r.Group("/")
	adminGroup.Use(AuthMiddleware(), RequireRole("admin"))
	{
		adminGroup.POST("/admin/plans", handlers.CreatePlan(services.Plan))
		adminGroup.PUT("/admin/plans/:id", handlers.UpdatePlan(services.Plan))
		adminGroup.DELETE("/admin/plans/:id", handlers.DeactivatePlan(services.Plan))
		adminGroup.GET("/admin/promo-codes", handlers.ListPromos(services.Promo))
		adminGroup.POST("/admin/promo-codes", handlers.CreatePromo(services.Promo))
		adminGroup.PUT("/admin/promo-codes/:id", handlers.UpdatePromo(services.Promo))
		adminGroup.DELETE("/admin/promo-codes/:id", handlers.DeletePromo(services.Promo))
	}

	// internal endpoints
	internalGroup := r.Group("/")
	internalGroup.Use(InternalAPIKeyMiddleware())
	{
		internalGroup.GET("/internal/quotas/:userId", handlers.GetQuotas(services.Plan))
	}

	// Fallback
	r.NoRoute(handlers.NoRoute)
}

package services

import (
	"net/http"
	"time"

	"9itchi-payments-api/core"
	"9itchi-payments-api/pkg/account"
	"9itchi-payments-api/repositories"
	"9itchi-payments-api/utils"
)

// Services holds service implementations and the peer clients they call.
type Services struct {
	// Erase removes what this service keeps about a deleted account.
	Erase        EraseService
	Plan         PlanService
	Promo        PromoService
	Subscription SubscriptionService
	Issuer       IssuerService
	Gift         GiftService
	Sale         SaleService
	Student      StudentDiscountService
	FreeGrant    FreeGrantService
	Clients      *Clients
}

// Clients holds one HTTP client per peer service (singletons, 50s timeout).
type Clients struct {
	Account *account.Client
}

func InitServices(config utils.AppConfig, repos *repositories.Repositories) *Services {
	providers := core.NewProviderFactory(config, &http.Client{Timeout: core.FlouciHTTPTimeout})
	// Everything that charges or shows a price to a student reads the plans at today's price; the admin service
	// keeps the plans as they were typed.
	priced := PricedPlans(repos.Plan, repos.Sale, time.Now)
	events := core.NewPublisher(config.RabbitMQURL)
	accounts := account.New(config.AccountAPIURL, config.InternalAPIKey)
	// The free usage given to emails is laid over the Free plan wherever a student's plan is read.
	grants := &freeGrants{repo: repos.FreeGrant, users: accounts}
	plans := NewPlanService(repos.Plan, priced, repos.Subscription).(*planService)
	plans.grants = grants
	subs := NewSubscriptionService(repos.Subscription, priced, repos.Promo, repos.Student, repos.Payment, repos.Store, providers, events).(*subscriptionService)
	subs.grants = grants
	return &Services{
		Erase:        NewEraseService(repos.Erase),
		Plan:         plans,
		Promo:        NewPromoService(repos.Promo, priced),
		Subscription: subs,
		Issuer:       NewIssuerService(repos.Issuer),
		Gift:         NewGiftService(repos.Gift, repos.Subscription, priced, repos.Store, events),
		Sale:         NewSaleService(repos.Sale, repos.Plan),
		Student:      NewStudentDiscountService(repos.Student),
		FreeGrant:    NewFreeGrantService(repos.FreeGrant),
		Clients: &Clients{
			Account: accounts,
		},
	}
}

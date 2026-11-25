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
	Plan         PlanService
	Promo        PromoService
	Subscription SubscriptionService
	Issuer       IssuerService
	Gift         GiftService
	Sale         SaleService
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
	return &Services{
		Plan:         NewPlanService(repos.Plan, priced, repos.Subscription),
		Promo:        NewPromoService(repos.Promo, priced),
		Subscription: NewSubscriptionService(repos.Subscription, priced, repos.Promo, repos.Payment, repos.Store, providers, events),
		Issuer:       NewIssuerService(repos.Issuer),
		Gift:         NewGiftService(repos.Gift, repos.Subscription, priced, repos.Store, events),
		Sale:         NewSaleService(repos.Sale, repos.Plan),
		Clients: &Clients{
			Account: account.New(config.AccountAPIURL, config.InternalAPIKey),
		},
	}
}

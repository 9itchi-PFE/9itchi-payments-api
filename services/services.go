package services

import (
	"net/http"

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
	Clients      *Clients
}

// Clients holds one HTTP client per peer service (singletons, 50s timeout).
type Clients struct {
	Account *account.Client
}

func InitServices(config utils.AppConfig, repos *repositories.Repositories) *Services {
	providers := core.NewProviderFactory(config, &http.Client{Timeout: core.FlouciHTTPTimeout})
	return &Services{
		Plan:         NewPlanService(repos.Plan, repos.Subscription),
		Promo:        NewPromoService(repos.Promo, repos.Plan),
		Subscription: NewSubscriptionService(repos.Subscription, repos.Plan, repos.Promo, repos.Payment, repos.Store, providers, core.LogPublisher{}),
		Issuer:       NewIssuerService(repos.Issuer),
		Clients: &Clients{
			Account: account.New(config.AccountAPIURL, config.InternalAPIKey),
		},
	}
}

package services

import (
	"9itchi-payments-api/pkg/account"
	"9itchi-payments-api/repositories"
	"9itchi-payments-api/utils"
)

// Services holds service implementations and the peer clients they call.
type Services struct {
	Plan         PlanService
	Promo        PromoService
	Subscription SubscriptionService
	Clients      *Clients
}

// Clients holds one HTTP client per peer service (singletons, 50s timeout).
type Clients struct {
	Account *account.Client
}

func InitServices(config utils.AppConfig, repos *repositories.Repositories) *Services {
	return &Services{
		Plan:         NewPlanService(repos.Plan),
		Promo:        NewPromoService(repos.Promo),
		Subscription: NewSubscriptionService(repos.Subscription),
		Clients: &Clients{
			Account: account.New(config.AccountAPIURL, config.InternalAPIKey),
		},
	}
}

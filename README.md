# 9itchi-payments-api

Subscription plans with quotas, promo codes, subscriptions and payments. Provider factory (Stripe or local gateway).

Port `8089`, health `/health`. Plans, promo codes, quotas and `GET /subscriptions/me` are implemented. Checkout, cancel and webhooks return `501 not implemented` until a payment provider is wired.

The migration seeds a `Free` plan (price 0, 36500 days, `emails_per_month` 20, `resumes` 3). It is the fallback for users with no active subscription and cannot be renamed or deactivated.

## Endpoints

| Method | Path | Access | Handler |
|---|---|---|---|
| GET | `/plans` | public | ListPlans (active only) |
| GET | `/admin/plans` | admin | ListAllPlans |
| POST | `/admin/plans` | admin | CreatePlan |
| PUT | `/admin/plans/:id` | admin | UpdatePlan |
| DELETE | `/admin/plans/:id` | admin | DeactivatePlan (soft, active=false) |
| GET | `/internal/quotas/:userId` | internal | GetQuotas (active plan, else Free) |
| POST | `/promo-codes/validate` | user | ValidatePromo (returns valid, discount_amount, final_price, reason) |
| GET | `/admin/promo-codes` | admin | ListPromos |
| POST | `/admin/promo-codes` | admin | CreatePromo |
| PUT | `/admin/promo-codes/:id` | admin | UpdatePromo |
| DELETE | `/admin/promo-codes/:id` | admin | DeletePromo |
| POST | `/subscriptions/checkout` | user | Checkout (501) |
| GET | `/subscriptions/me` | user | MySubscription (active plan, else Free) |
| POST | `/subscriptions/cancel` | user | CancelSubscription (501) |
| POST | `/webhooks/:provider` | public | Webhook (501) |

## Run

```
cp .env.example .env
go run main.go
```

## Tests

```
go test ./...
go test -tags integration -count=1 ./repositories/...   # embedded Postgres, ~1 min
```

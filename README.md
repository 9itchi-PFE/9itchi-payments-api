# 9itchi-payments-api

Subscription plans with quotas, promo codes, subscriptions and payments. Provider factory (Stripe or local gateway).

Port `8089`, health `/health`. Skeleton only: every endpoint returns `501 not implemented` until built.

## Endpoints

| Method | Path | Access | Handler |
|---|---|---|---|
| GET | `/plans` | public | ListPlans |
| POST | `/admin/plans` | admin | CreatePlan |
| PUT | `/admin/plans/:id` | admin | UpdatePlan |
| DELETE | `/admin/plans/:id` | admin | DeactivatePlan |
| GET | `/internal/quotas/:userId` | internal | GetQuotas |
| POST | `/promo-codes/validate` | user | ValidatePromo |
| GET | `/admin/promo-codes` | admin | ListPromos |
| POST | `/admin/promo-codes` | admin | CreatePromo |
| PUT | `/admin/promo-codes/:id` | admin | UpdatePromo |
| DELETE | `/admin/promo-codes/:id` | admin | DeletePromo |
| POST | `/subscriptions/checkout` | user | Checkout |
| GET | `/subscriptions/me` | user | MySubscription |
| POST | `/subscriptions/cancel` | user | CancelSubscription |
| POST | `/webhooks/:provider` | public | Webhook |

## Run

```
cp .env.example .env
go run main.go
```

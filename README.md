# 9itchi-payments-api

Subscription plans with quotas, promo codes, subscriptions and payments. Provider factory (Stripe or local gateway).

Port `8089`, health `/health`. Plans, promo codes, quotas, `GET /subscriptions/me`, checkout, cancel and the Flouci webhook are implemented.

Payments go through the `flouci` provider (`core/flouci.go`, registered in `core.NewProviderFactory`). Checkout stores a pending row in `payments` (amounts in integer millimes), calls Flouci and returns `{payment_url, payment_id}`. The webhook verifies the HMAC signature (`X-Flouci-Signature`), re-checks the payment with Flouci, then in one transaction completes the payment (replays are no-ops), creates or extends the subscription and records promo usage; `payment.completed` / `subscription.created` go through `core.Publisher`. A 0 TND checkout (100% promo) activates directly without a provider call; the Free plan itself needs no checkout (400). Cancel is immediate (the Free plan applies again). Without `FLOUCI_PUBLIC_KEY` / `FLOUCI_PRIVATE_KEY`, checkout answers `503 payment provider unavailable`.

Config (all optional): `FLOUCI_BASE_URL` (default `https://developers.flouci.com`), `FLOUCI_PUBLIC_KEY`, `FLOUCI_PRIVATE_KEY`, `FLOUCI_WEBHOOK_SECRET`, `FLOUCI_WEBHOOK_URL` (public URL of `/webhooks/flouci`, skipped when local), `FRONTEND_URL` (default `http://localhost:3000`).

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
| POST | `/subscriptions/checkout` | user | Checkout (plan_id, optional promo_code; returns payment_url, payment_id) |
| GET | `/subscriptions/me` | user | MySubscription (active plan, else Free) |
| POST | `/subscriptions/cancel` | user | CancelSubscription (active paid subscription, 404 if none) |
| POST | `/webhooks/:provider` | public | Webhook (signature verified; 404 unknown provider, 401 bad signature) |

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

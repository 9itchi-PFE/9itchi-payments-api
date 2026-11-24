# 9itchi-payments-api

Subscription plans with quotas, promo codes, subscriptions and payments. Provider factory (Stripe or local gateway).

Port `8089`, health `/health`. Plans, promo codes, quotas, `GET /subscriptions/me`, checkout, cancel and the Flouci webhook are implemented.

Payments go through the `flouci` provider (`core/flouci.go`, registered in `core.NewProviderFactory`). Checkout stores a pending row in `payments` (amounts in integer millimes), calls Flouci and returns `{payment_url, payment_id}`. The webhook verifies the HMAC signature (`X-Flouci-Signature`), re-checks the payment with Flouci, then in one transaction completes the payment (replays are no-ops), creates or extends the subscription and records promo usage; `payment.completed` / `subscription.created` go through `core.Publisher`. A 0 TND checkout (100% promo) activates directly without a provider call; the Free plan itself needs no checkout (400). A subscription renews every period unless the student stops it, but Flouci cannot charge by itself: "renews" means a new payment is asked when the period ends, and the student pays it. `POST /subscriptions/cancel` stops the renewal (`stopped_at`); access lasts until `ends_at` and no payment is asked afterwards. `POST /subscriptions/resume` undoes it, and so does paying again. `GET /subscriptions/me` gives `subscription.renews`, and on the Free view `lapsed`: the last subscription when it ended unpaid without having been stopped. Without `FLOUCI_PUBLIC_KEY` / `FLOUCI_PRIVATE_KEY`, checkout answers `503 payment provider unavailable`.

Config (all optional): `FLOUCI_BASE_URL` (default `https://developers.flouci.com`), `FLOUCI_PUBLIC_KEY`, `FLOUCI_PRIVATE_KEY`, `FLOUCI_WEBHOOK_SECRET`, `FLOUCI_WEBHOOK_URL` (public URL of `/webhooks/flouci`, skipped when local), `FRONTEND_URL` (default `http://localhost:3000`).

The migrations seed a `Free` plan (price 0, 36500 days). Its quotas follow the product decision: `emails_per_month` 0 (no sending through 9itchi), `resumes` 1 (one CV version) and `cv_downloads` 1 (one free download, with the 9itchi logo). Paid plans are created by the admin (prices in TND). It is the fallback for users with no active subscription and cannot be renamed or deactivated.

After paying, Flouci sends the browser back to `FRONTEND_URL/settings/billing?payment=success|failed&id=<payment_id>`. The query string is never trusted: the page calls `POST /payments/:id/verify`, which asks Flouci whether a pending payment is paid and completes it exactly like the webhook does (same transaction, replays are no-ops). The return page therefore works even when the webhook is late or cannot reach a local machine.

A completed payment gets a receipt number (`receipt_no`, shown as `<year>-<6 digits>`) and the period it paid for (`period_start`, `period_end`; when it extends a running subscription the period starts where the current one ends). `GET /payments` returns them with the plan name, the promo code and the amounts in millimes.

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
| POST | `/subscriptions/cancel` | user | CancelSubscription (stops the renewal, access until the period ends; 404 if no active paid subscription) |
| POST | `/subscriptions/resume` | user | ResumeSubscription (renews again; 404 if none) |
| GET | `/payments` | user | ListPayments (completed payments, newest first: the receipts) |
| POST | `/payments/:id/verify` | user | VerifyPayment (where a payment stands; a pending one is re-checked with Flouci) |
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

# 9itchi-payments-api

Subscription plans with quotas, promo codes, subscriptions and payments. Provider factory (Stripe or local gateway).

Port `8089`, health `/health`. Plans, promo codes, quotas, `GET /subscriptions/me`, checkout, cancel and the Flouci webhook are implemented.

Payments go through the `flouci` provider (`core/flouci.go`, registered in `core.NewProviderFactory`). Checkout stores a pending row in `payments` (amounts in integer millimes), calls Flouci and returns `{payment_url, payment_id}`. The webhook verifies the HMAC signature (`X-Flouci-Signature`), re-checks the payment with Flouci, then in one transaction completes the payment (replays are no-ops), creates or extends the subscription and records promo usage; `payment.completed` / `subscription.created` go through `core.Publisher`. A 0 TND checkout (100% promo) activates directly without a provider call; the Free plan itself needs no checkout (400). A subscription renews every period unless the student stops it, but Flouci cannot charge by itself: "renews" means a new payment is asked when the period ends, and the student pays it. `POST /subscriptions/cancel` stops the renewal (`stopped_at`); access lasts until `ends_at` and no payment is asked afterwards. `POST /subscriptions/resume` undoes it, and so does paying again. `GET /subscriptions/me` gives `subscription.renews`, and on the Free view `lapsed`: the last subscription when it ended unpaid without having been stopped. Without `FLOUCI_PUBLIC_KEY` / `FLOUCI_PRIVATE_KEY`, checkout answers `503 payment provider unavailable`.

Config (all optional): `FLOUCI_BASE_URL` (default `https://developers.flouci.com`), `FLOUCI_PUBLIC_KEY`, `FLOUCI_PRIVATE_KEY`, `FLOUCI_WEBHOOK_SECRET`, `FLOUCI_WEBHOOK_URL` (public URL of `/webhooks/flouci`, skipped when local), `FRONTEND_URL` (default `http://localhost:3000`).

The migrations seed a `Free` plan (price 0, 36500 days). Its quotas follow the product decision: `emails_per_month` 0 (no sending through 9itchi), `resumes` 1 (one CV version) and `cv_downloads` 1 (one free download, with the 9itchi logo). Paid plans are created by the admin (prices in TND). It is the fallback for users with no active subscription and cannot be renamed or deactivated.

After paying, Flouci sends the browser back to `FRONTEND_URL/settings/billing?payment=success|failed&id=<payment_id>`. The query string is never trusted: the page calls `POST /payments/:id/verify`, which asks Flouci whether a pending payment is paid and completes it exactly like the webhook does (same transaction, replays are no-ops). The return page therefore works even when the webhook is late or cannot reach a local machine.

A completed payment gets a receipt number (`receipt_no`, shown as `<year>-<6 digits>`) and the period it paid for (`period_start`, `period_end`; when it extends a running subscription the period starts where the current one ends). `GET /payments` returns them with the plan name, the promo code and the amounts in millimes.

A plan can carry a `regular_price`: what it normally costs, shown struck through next to `price`, which stays the amount charged. It must be above `price`. `PUT /admin/plans/:id` replaces the whole plan, so leaving `regular_price` out removes it. A payment keeps the regular price of the day it was opened (`regular_millimes`), so its receipt goes on showing the difference after the plan changes.

The launch price can have a last day, `launch_ends_on` (`YYYY-MM-DD`, set by the admin with the plan; it needs a `regular_price`). From the next day in Tunis the plan costs its regular price for everyone: `GET /plans`, the checkout, the promo codes and `GET /subscriptions/me` then return `price` = the regular price, with no `regular_price` and no `launch_ends_on`. Before that, `GET /plans` carries `launch_ends_on` so the student can be told until when. `GET /admin/plans` always returns the plan as it was typed. A payment already opened keeps its amount. A renewal reminder names the price of the day the period ends.

The legal information printed on receipts (`legal_name`, `address`, `tax_id`, `vat_mention`) lives in one row of `receipt_issuer`, seeded empty. An empty field is simply not known yet: the receipt shows a blank for it, and `complete` turns true once all four are filled in. The admin fills them in with `PUT /admin/receipt-issuer` (full replacement, empty values allowed). Receipts always print the current values. `vat_mention` is free text (for example the wording the accountant gives); nothing is computed from it.

## Endpoints

| Method | Path | Access | Handler |
|---|---|---|---|
| GET | `/plans` | public | ListPlans (active only) |
| GET | `/admin/plans` | admin | ListAllPlans |
| POST | `/admin/plans` | admin | CreatePlan |
| PUT | `/admin/plans/:id` | admin | UpdatePlan |
| DELETE | `/admin/plans/:id` | admin | DeactivatePlan (soft, active=false) |
| GET | `/internal/quotas/:userId` | internal | GetQuotas (active plan, else Free) |
| POST | `/internal/subscriptions/renewal-reminders` | internal | SendRenewalReminders (raises `subscription.renewal_due`, once per period; returns `due`, `lapsed`, `failed`) |
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
| GET | `/receipt-issuer` | user | GetReceiptIssuer (legal information for receipts; empty fields are not known yet) |
| PUT | `/admin/receipt-issuer` | admin | UpdateReceiptIssuer (full replacement) |
| GET | `/admin/users/:id/subscription` | admin | UserSubscription (the plan and subscription of one student, as `GET /subscriptions/me`) |
| GET | `/admin/users/:id/payments` | admin | UserPayments (the completed payments of one student, as `GET /payments`) |
| POST | `/webhooks/:provider` | public | Webhook (signature verified; 404 unknown provider, 401 bad signature) |

## Renewal reminders

Nothing is charged automatically, so the student is reminded. `POST /internal/subscriptions/renewal-reminders` (called by schedule-api) raises `subscription.renewal_due` for the renewing subscriptions of a paid plan:

- `stage: "due"`: the period ends within 7 days (the same 7 days as the banner of the app);
- `stage: "lapsed"`: the period ended within the last 7 days, was not renewed, and the user has no running subscription.

The payload carries `user_id`, `subscription_id`, `plan_id`, `plan_name`, `price_millimes`, `ends_at` and `stage`. A subscription is reminded once per stage and per period (`due_reminded_for`, `lapse_reminded_for` keep the end of the period that was reminded); a renewal moves the end, so the next period gets its own reminders. A subscription the student stopped is never reminded. The call can be repeated and run by several instances at once: each subscription is claimed by one. When the event cannot be published the claim is given back and the next call tries again. Without `RABBITMQ_URL` the event is only logged and the subscription still counts as reminded.

## Events (RabbitMQ)

Optional. With `RABBITMQ_URL` set, `payment.completed` and, for a first period, `subscription.created` (`user_id`, `plan_id`, `payment_id`, `amount_millimes`, and what the receipt says: `receipt_no`, `plan_name`, `duration_days`, `period_start`, `period_end`) are published to the topic exchange `9itchi.events` (routing key = event name) as persistent JSON messages, each confirmed by the broker and carrying a message id consumers deduplicate on. Without the variable the event name is only logged.

A publish failure is logged and never fails the request. A broker that cannot be reached is left alone for 15 s, so it costs one short wait and not one per request; events raised meanwhile are lost (there is no outbox). Broker tests: `RABBITMQ_TEST_URL=amqp://guest:guest@localhost:5672/ go test -tags integration ./core/` (skipped without the variable).

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

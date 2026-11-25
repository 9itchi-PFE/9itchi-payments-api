# 9itchi-payments-api

Subscription plans with quotas, promo codes, subscriptions and payments. Provider factory (Stripe or local gateway).

Port `8089`, health `/health`. Plans, promo codes, quotas, `GET /subscriptions/me`, checkout, cancel and the Flouci webhook are implemented.

Payments go through the `flouci` provider (`core/flouci.go`, registered in `core.NewProviderFactory`). Checkout stores a pending row in `payments` (amounts in integer millimes), calls Flouci and returns `{payment_url, payment_id}`. The webhook verifies the HMAC signature (`X-Flouci-Signature`), re-checks the payment with Flouci, then in one transaction completes the payment (replays are no-ops), creates or extends the subscription and records promo usage; `payment.completed` / `subscription.created` go through `core.Publisher`. A 0 TND checkout (100% promo) activates directly without a provider call; the Free plan itself needs no checkout (400). A subscription renews every period unless the student stops it, but Flouci cannot charge by itself: "renews" means a new payment is asked when the period ends, and the student pays it. `POST /subscriptions/cancel` stops the renewal (`stopped_at`); access lasts until `ends_at` and no payment is asked afterwards. `POST /subscriptions/resume` undoes it, and so does paying again. `GET /subscriptions/me` gives `subscription.renews`, and on the Free view `lapsed`: the last subscription when it ended unpaid without having been stopped. Without `FLOUCI_PUBLIC_KEY` / `FLOUCI_PRIVATE_KEY`, checkout answers `503 payment provider unavailable`.

Config (all optional): `FLOUCI_BASE_URL` (default `https://developers.flouci.com`), `FLOUCI_PUBLIC_KEY`, `FLOUCI_PRIVATE_KEY`, `FLOUCI_WEBHOOK_SECRET`, `FLOUCI_WEBHOOK_URL` (public URL of `/webhooks/flouci`, skipped when local), `FRONTEND_URL` (default `http://localhost:3000`).

The migrations seed a `Free` plan (price 0, 36500 days). Its quotas follow the product decision of 25 Nov. (`0015_free_plan_minimal.sql`): 3 `free_applications` with the imported CV, no CV editing (`cv_edit` 0), no download, no AI and no CV check. Paid plans are created by the admin (prices in TND). It is the fallback for users with no active subscription and cannot be renamed or deactivated.

Every other plan is a paid plan: its `price` must be above 0 (a free plan could never be bought, checkout refuses it) and its name cannot be `Free` in any case. Names are unique. `DELETE /admin/plans/:id` switches a plan off: it leaves `GET /plans` and checkout refuses it, so its subscribers keep their access until the end of the period they paid but cannot renew on it. `PUT /admin/plans/:id` with `active: true` switches it on again.

After paying, Flouci sends the browser back to `FRONTEND_URL/settings/billing?payment=success|failed&id=<payment_id>`. The query string is never trusted: the page calls `POST /payments/:id/verify`, which asks Flouci whether a pending payment is paid and completes it exactly like the webhook does (same transaction, replays are no-ops). The return page therefore works even when the webhook is late or cannot reach a local machine.

A completed payment gets a receipt number (`receipt_no`, shown as `<year>-<6 digits>`) and the period it paid for (`period_start`, `period_end`; when it extends a running subscription the period starts where the current one ends). Paying for another plan while one is running (a monthly student taking the yearly plan) starts the new plan at once and adds the unused days of the old one at its end, so no paid day is lost. `GET /payments` returns them with the plan name, the promo code and the amounts in millimes.

A plan can carry a `regular_price`: what it normally costs, shown struck through next to `price`, which stays the amount charged. It must be above `price`. `PUT /admin/plans/:id` replaces the whole plan, so leaving `regular_price` out removes it. A payment keeps the regular price of the day it was opened (`regular_millimes`), so its receipt goes on showing the difference after the plan changes.

The launch price can have a last day, `launch_ends_on` (`YYYY-MM-DD`, set by the admin with the plan; it needs a `regular_price`). From the next day in Tunis the plan costs its regular price for everyone: `GET /plans`, the checkout, the promo codes and `GET /subscriptions/me` then return `price` = the regular price, with no `regular_price` and no `launch_ends_on`. Before that, `GET /plans` carries `launch_ends_on` so the student can be told until when. `GET /admin/plans` always returns the plan as it was typed. A payment already opened keeps its amount. A renewal reminder names the price of the day the period ends.

**The launch price is kept by those who paid it** (migration `0014`). A payment at a launch price records it (`payments.launch_millimes`, before any promo code) and the subscription it creates keeps it (`subscriptions.launch_millimes`). When that student pays the same plan again, before the end of the period or at most 7 days after it, the checkout charges the kept launch price even once the launch is over; the receipt strikes today's price through. A sale cheaper than the kept price wins that time, and the launch price is still carried on. A gap of more than 7 days, or another plan, means today's price. `GET /subscriptions/me` returns `kept_price` (null when none) and, when it is lower than today's price, the plan at that price with today's price as `regular_price`; on the lapsed view `kept_price` is set while the 7 days run. Renewal reminders name the kept price.

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
| GET | `/admin/sales` | admin | ListSales |
| POST | `/admin/sales` | admin | CreateSale |
| PUT | `/admin/sales/:id` | admin | UpdateSale |
| DELETE | `/admin/sales/:id` | admin | DeleteSale |
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
| GET | `/admin/users/:id/gifts` | admin | ListGifts (the months the team gave one student, newest first) |
| POST | `/admin/users/:id/gifts` | admin | GiveMonth (30 days for free; 409 within a minute of the last one, or when no paid plan is on sale) |
| POST | `/webhooks/:provider` | public | Webhook (signature verified; 404 unknown provider, 401 bad signature) |

## What a plan allows (`quotas`)

`quotas` is a free map of limits read by the other services through `GET /internal/quotas/:userId`, which also gives `period_start`, the start of the current period of the subscription (null without one). The keys in use:

| Key | Read by | Meaning |
|---|---|---|
| `resumes` | resume-api | how many CVs |
| `cv_downloads` | resume-api | downloads of a CV per period of the subscription; `0` allows none |
| `cv_edit` | resume-api | `0`: the CVs can be imported and read, not edited in 9itchi (Free) |
| `ats_contact`, `ats_summary`, `ats_experience`, `ats_education`, `ats_skills`, `ats_layout`, `ats_file` | resume-api | `0` hides that group of the CV diagnosis |
| `emails_per_month` | email-api | applications sent by 9itchi each month, with the CV built in 9itchi; `0` allows none |
| `free_applications` | email-api | when `emails_per_month` is `0`: applications 9itchi still sends, over the whole life of the account, with the file the student imported |
| `ai_drafts_per_month` | email-api | letters written by the AI in a calendar month (Free: 0) |
| `ai_rewrites_per_month` | resume-api | rewrites of a CV line or of the summary by the AI in a calendar month (Free: 0) |
| `ai_matches_per_month` | email-api | AI analyses of how well the CV fits an offer in a calendar month (Free: 0) |

A key that is absent sets no limit. **There is no free offer**: the plan named `Free` is what a student with no subscription gets (migration `0015`, decided on 25 November): browse the offers, import one CV and send three applications with the imported file, as it is. No diagnosis, no AI, no download, and the CV is not edited in 9itchi. It can be changed like any plan, in the admin.

## Sales: a reduction without a code, between two days

A sale (`sales`, migration `0012`) has a name students see, a percentage (1 to 90), a first and a last day (both included, counted in Tunis) and, optionally, the plans it applies to (none: every paid plan). The admin writes them at `/admin/sales`; each comes back with its `state` today (`upcoming`, `running`, `over`).

While a sale runs, every price a student is shown or charged is the price of the sale: `GET /plans` returns it in `price`, the usual price in `regular_price`, and `sale: {name, percent, ends_on}`. The checkout charges it, and the payment keeps the usual price (`regular_millimes`) and the name of the sale (`sale_name`) for the receipt. Nothing is stored on the plan: the day after the last day the plan costs what it cost before.

- Two sales the same day: the one that takes the most off wins; they do not add up.
- On a plan with a launch price the percentage is taken off the **regular price**, never the launch price. If that leaves a price above the launch price, the launch price stays and the plan is not shown as in a sale: a sale never makes a plan cost more.
- **No promo code during a sale**: `POST /promo-codes/validate` answers `valid: false` with the reason `promo codes cannot be used during a sale`, and a checkout with a code is refused (`400`).
- A subscription already paid does not change; a renewal paid during the sale costs the price of the sale.
- Deleting a running sale ends it at once.

## A month given by the team

`POST /admin/users/:id/gifts` gives the student 30 days, with no body: the admin is read from the token and kept with the gift (`subscription_gifts`).

- **A paid subscription is running**: it ends 30 days later, on the same plan. Its renewal stays as the student left it (a stopped one stays stopped), and the reminders follow the new end.
- **None is running**: 30 days of a paid plan start now. The plan is the one the student had last when it is still on sale, else the cheapest paid plan on sale. This subscription is stopped from the start: the student did not buy it, so no renewal is asked and no reminder is sent when it ends. `POST /subscriptions/resume` lets them keep it.

A gift is not a payment: no receipt, no amount, nothing in `GET /payments`, and no `payment.completed` (the revenue figures do not move). It raises `subscription.gifted` with `user_id`, `gift_id`, `plan_id`, `plan_name`, `days` and `ends_at`. A second gift to the same student within a minute is refused with 409, so a double click or two admins at once give one month. The call does not check that the user exists: it is made from the file of a student.

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

## Account deletion

`DELETE /internal/users/:id` (`x-api-key`) is called by account-api before it deletes an account. Here it concerns nothing: the payments and their receipts are accounting records and stay, under the id of the account only. What it does is stop the subscriptions, so that no renewal is asked for an account that no longer exists. It runs in one transaction, answers `204`, and can be called again without harm (account-api retries until every service has answered). Nothing about the user is logged.

# 9itchi Payments API
Plans and quotas (single source of truth for subscriptions), promo codes, sales, launch price, Flouci checkout, receipts, admin gifts, renewal reminders. Money in millimes (bigint, TND). Port 8089, DB `payments`.

## Conventions (shared by every 9itchi Go service)
- `handlers/` thin (validate, authorize, call service, JSON) -> `services/` all logic, returns `(T, *models.ApiError)` -> `repositories/` only Postgres, plain errors (`ErrNotFound`, `ErrConflict`). Error body `{"error": "..."}`.
- DB calls use `repositories.QueryTimeout`; multi-table writes `Store.WithTx`.
- Schema: add `migrations/NNNN_name.sql` (goose, embedded, runs at startup). Never edit a merged migration.
- Auth: `Authorization: Bearer` AES-GCM-wrapped HS256 JWT (claims `id`, `email`, `role`, `typ`, `exp`; access only). `/admin/*` = `RequireRole("admin")`. `/internal/*` = `x-api-key` (`INTERNAL_API_KEY`), blocked by nginx.
- Events: exchange `9itchi.events` (topic, routing key = event name), names in `core/events.go`; empty `RABBITMQ_URL` = `LogPublisher`.
- Env only, validated in `utils/config.go`. Common: `PORT`, `ENVIRONMENT`, `DATABASE_URL`, `ITCHI_SECRET_KEY`, `ITCHI_ENCRYPTION_KEY`, `INTERNAL_API_KEY`, `INTERNAL_API_KEY_REQUIRED`. Never log tokens, passwords, PII.
- Tests: `{resource}_test.go`, testify, func-field fakes (no testify/mock), success + error paths. Repo tests: `//go:build integration`, embedded Postgres.
- Check: `gofmt -l .; go build ./... && go vet ./... && go test ./...`; `go test -tags integration -count=1 ./repositories/...`; `golangci-lint run`.

## Routes
- Public: `GET /plans`, `POST /webhooks/:provider`.
- User: `POST /promo-codes/validate`; `POST /subscriptions/checkout|cancel|resume`, `GET /subscriptions/me`; `GET /payments`, `POST /payments/:id/verify`; `GET /receipt-issuer`.
- Admin: `/admin/plans` GET|POST, `/:id` PUT|DELETE (deactivate); `/admin/promo-codes` and `/admin/sales` same shape; `PUT /admin/receipt-issuer`; `GET /admin/users/:id/subscription|payments`; `GET|POST /admin/users/:id/gifts`.
- Internal: `GET /internal/quotas/:userId` (resume, email), `POST /internal/subscriptions/renewal-reminders` (schedule), `DELETE /internal/users/:id`.

## Data, events, providers
- Tables: `plans`, `promo_codes`, `sales`, `subscriptions`, `payments`, `receipt_issuer`, `subscription_gifts`. Latest `0015_free_plan_minimal.sql`.
- Publishes `payment.completed`, `subscription.created`, `subscription.renewal_due` (`due` | `lapsed`), `subscription.gifted`. Consumes nothing.
- `core.ProviderFactory` = `map[string]func() PaymentProvider`, only `flouci`; checkout 503 until keys set. Webhook signature constant-time, payment always re-verified with gateway, replays idempotent.
- Env: `RABBITMQ_URL`, `ACCOUNT_API_URL`, `FRONTEND_URL`, `FLOUCI_BASE_URL|PUBLIC_KEY|PRIVATE_KEY|WEBHOOK_SECRET|WEBHOOK_URL`.

## Domain rules
- Free plan = "no subscription"; `GET /subscriptions/me` falls back to it. Free (since `0015`): `free_applications` 3, `cv_edit` 0, `cv_downloads` 0, AI quotas 0, `ats_*` 0. All editable in admin Plans.
- Quota keys: `resumes`, `cv_downloads` (per period), `ats_<group>`, `cv_edit`, `ai_{drafts,rewrites,matches}_per_month`, `free_applications` (account lifetime), `emails_per_month`. Missing key = unlimited.
- Pricing (`services/pricing.go`, Tunis days): launch price until `launch_ends_on`, then `regular_price`. Sale % applies to the regular price, never the launch price, never raises the price, never on Free. No promo code during a sale; best of two sales wins.
- Kept launch price: same-plan renewal keeps `launch_millimes` if the gap <= 7 days (`KeepGap`).
- No auto-charge: student pays each period. Cancel stops renewal, access until `ends_at`; resume only before end. 0 TND checkout skips the gateway (`activateFree`).
- Gift: 30 days, no payment/receipt; extends a running subscription, else last (or cheapest) plan from now; double-gift guard.
- Renewal reminders: 7 days before end and 7 days after lapse, once per period.
- Refunds are manual at Flouci (7-day rule, pending legal check); no refund code here.

## Gotchas
- `pkg/account` client is an unused stub. Real Flouci merchant account not yet available.

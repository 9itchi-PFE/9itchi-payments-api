# 9itchi Payments API
Plans and quotas (single source of truth for subscriptions), promo codes, sales, launch price, Flouci checkout, receipts, admin gifts, renewal reminders. Money in millimes (bigint, TND). Port 8089, DB `payments`.

## Conventions (shared by every 9itchi Go service)
- `handlers/` thin (validate, authorize, call service, JSON) -> `services/` all logic, returns `(T, *models.ApiError)` -> `repositories/` only Postgres, plain errors (`ErrNotFound`, `ErrConflict`). Error body `{"error": "..."}`.
- DB calls use `repositories.QueryTimeout`; multi-table writes `Store.WithTx`.
- Schema: add `migrations/NNNN_name.sql` (goose, embedded, runs at startup). Never edit a merged migration.
- Auth: `Authorization: Bearer` AES-GCM-wrapped HS256 JWT (claims `id`, `email`, `role`, `typ`, `exp`, `email_verified`; access only). `/admin/*` = `RequireRole("admin")`. `/internal/*` = `x-api-key` (`INTERNAL_API_KEY`), blocked by nginx.
- Events: exchange `9itchi.events` (topic, routing key = event name), names in `core/events.go`; empty `RABBITMQ_URL` = `LogPublisher`.
- Env only, validated in `utils/config.go`. Common: `PORT`, `ENVIRONMENT`, `DATABASE_URL`, `ITCHI_SECRET_KEY`, `ITCHI_ENCRYPTION_KEY`, `INTERNAL_API_KEY`, `INTERNAL_API_KEY_REQUIRED`. Never log tokens, passwords, PII.
- Tests: `{resource}_test.go`, testify, func-field fakes (no testify/mock), success + error paths. Repo tests: `//go:build integration`, embedded Postgres.
- Check: `gofmt -l .; go build ./... && go vet ./... && go test ./...`; `go test -tags integration -count=1 ./repositories/...`; `golangci-lint run`.

## Routes
- Public: `GET /plans`, `POST /webhooks/:provider`.
- User: `POST /promo-codes/validate`; `POST /subscriptions/checkout|cancel|resume`, `GET /subscriptions/me`; `GET /payments`, `POST /payments/:id/verify`; `GET /receipt-issuer`; `GET /student-discount/me`.
- Admin: `/admin/plans` GET|POST, `/:id` PUT|DELETE (deactivate); `/admin/promo-codes` and `/admin/sales` same shape; `PUT /admin/receipt-issuer`; `GET|PUT /admin/student-discount`; `GET /admin/users/:id/subscription|payments`; `GET|POST /admin/users/:id/gifts`; `/admin/free-grants` GET|POST (many emails, same quotas), `/:email` PUT|DELETE.
- Internal: `GET /internal/quotas/:userId` (resume, email), `POST /internal/subscriptions/renewal-reminders` (schedule), `DELETE /internal/users/:id`.

## Data, events, providers
- Tables: `plans`, `promo_codes`, `sales`, `subscriptions`, `payments`, `receipt_issuer`, `subscription_gifts`, `student_discount` (one row), `free_grants` (email PK, lower-case). Latest `0021_free_grants.sql`.
- Publishes `payment.completed`, `subscription.created`, `subscription.renewal_due` (`due` | `lapsed`), `subscription.gifted`. Consumes nothing.
- `core.ProviderFactory` = `map[string]func() PaymentProvider`, only `flouci`; checkout 503 until keys set. Webhook signature constant-time, payment always re-verified with gateway, replays idempotent.
- Calls account-api `GET /internal/users/:id` (email, `email_verified`) for free grants.
- Env: `RABBITMQ_URL`, `ACCOUNT_API_URL`, `FRONTEND_URL`, `FLOUCI_BASE_URL|PUBLIC_KEY|PRIVATE_KEY|WEBHOOK_SECRET|WEBHOOK_URL`.

## Domain rules
- Admin plans: every plan but Free must cost > 0 and cannot be named Free (any case); names unique (409). `DELETE` switches a plan off (gone from `GET /plans`, checkout 404, subscribers keep access to `ends_at`); `PUT active: true` switches it on. `duration_days` cannot change while the plan has active subscribers (409, message contains "length"): quota periods count with it.
- Plan change (e.g. monthly -> yearly): new plan starts now, unused days of the old one are added at its end. Same plan: extends from current end.
- Free plan = "no subscription"; `GET /subscriptions/me` falls back to it. Free (since `0021`, 28 Nov. 2026): every quota 0 (`resumes`, `free_applications`, `ai_scores_per_day`, AI, `ats_*`, `cv_*`, `recruiter_searches_per_day`): only reading offers. Paid plans: `recruiter_searches_per_day` 6, `ai_scores_per_day` 15. All editable in admin Plans.
- Free grants (`services/grant.go`): admin gives quotas to emails; while the user has no subscription, `GET /internal/quotas/:userId` and `/subscriptions/me` lay them over Free key by key (plan name stays Free, `/me` sets `granted`). Only when account-api says the email is `email_verified` (Google/LinkedIn sign-in). Account-api or grant read failure = plain Free (logged). Counting periods are Free ones (e.g. `free_applications` lifetime).
- Plan `details` (jsonb, `fr`/`en`/`ar`: label <= 40, tagline <= 120, <= 10 features of 1..140 runes) is the text students read; any fault is a 400. Nil on `PUT` keeps the current details, `{}` clears them; Free seeded in `0017`.
- Quota keys: `resumes`, `cv_downloads` (per period), `ats_<group>`, `cv_edit`, `ai_{drafts,rewrites,matches}_per_month`, `free_applications` (account lifetime), `emails_per_month`, `recruiter_searches_per_day` and `ai_scores_per_day` (per Tunis day, read by email-api). Missing key = unlimited: admin-created plans copy quotas from an existing plan, so a plan made without the key has unlimited recruiter searches.
- Pricing (`services/pricing.go`, Tunis days): launch price until `launch_ends_on`, then `regular_price`. Sale % applies to the regular price, never the launch price, never raises the price, never on Free. No promo code during a sale; best of two sales wins.
- Student discount (`services/student.go`): admin sets one percent (0 = off, max 100) and a list of email domains (subdomains count). Applies at checkout only when the JWT email is on a listed domain AND `email_verified` (Google/LinkedIn sign-in, set by account-api); password-only accounts never get it. Stacks on top: % of the price left after launch/sale/promo. Stored as `payments.student_percent`, amount added to `discount_millimes`.
- Kept launch price: same-plan renewal keeps `launch_millimes` if the gap <= 7 days (`KeepGap`).
- No auto-charge: student pays each period. Cancel stops renewal, access until `ends_at`; resume only before end. 0 TND checkout skips the gateway (`activateFree`).
- Gift: 30 days, no payment/receipt; extends a running subscription, else last (or cheapest) plan from now; double-gift guard.
- Renewal reminders: 7 days before end and 7 days after lapse, once per period.
- Refunds are manual at Flouci (7-day rule, pending legal check); no refund code here.

## Gotchas
- Real Flouci merchant account not yet available.

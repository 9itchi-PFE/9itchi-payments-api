-- +goose Up
-- What the AI does for a student is limited per calendar month. Without a subscription: one letter, one rewrite
-- of a CV line or of the summary, three analyses of an offer. The paid plans set their own in the admin
-- (Plans); a plan that leaves a key out has no limit for it.
UPDATE plans
SET quotas = quotas || '{"ai_drafts_per_month": 1, "ai_rewrites_per_month": 1, "ai_matches_per_month": 3}'::jsonb, updated_at = now()
WHERE name = 'Free';

-- +goose Down
UPDATE plans
SET quotas = quotas - 'ai_drafts_per_month' - 'ai_rewrites_per_month' - 'ai_matches_per_month', updated_at = now()
WHERE name = 'Free';

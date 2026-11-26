-- +goose Up
-- Daily AI match scores on « Pour toi » (email-api), counted per Tunis day. Three a day without a subscription,
-- fifteen on every paid plan. A plan that leaves the key out has no limit for it.
UPDATE plans SET quotas = quotas || '{"ai_scores_per_day": 3}'::jsonb, updated_at = now() WHERE lower(name) = 'free';
UPDATE plans SET quotas = quotas || '{"ai_scores_per_day": 15}'::jsonb, updated_at = now() WHERE lower(name) <> 'free';

-- +goose Down
UPDATE plans SET quotas = quotas - 'ai_scores_per_day', updated_at = now();

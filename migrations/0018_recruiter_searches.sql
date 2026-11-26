-- +goose Up
-- Recruiter Radar (email-api) finds recruiters for a student, counted per Tunis day. Not without a subscription;
-- six a day on every paid plan, active or not. A plan that leaves the key out has no limit for it.
UPDATE plans SET quotas = quotas || '{"recruiter_searches_per_day": 0}'::jsonb, updated_at = now() WHERE lower(name) = 'free';
UPDATE plans SET quotas = quotas || '{"recruiter_searches_per_day": 6}'::jsonb, updated_at = now() WHERE lower(name) <> 'free';

-- +goose Down
UPDATE plans SET quotas = quotas - 'recruiter_searches_per_day', updated_at = now();

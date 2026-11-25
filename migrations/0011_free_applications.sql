-- +goose Up
-- Without a subscription a student can still apply through 9itchi a few times, with the file they imported and
-- not the CV built here. free_applications is that number, counted over the whole life of the account.
UPDATE plans
SET quotas = quotas || '{"free_applications": 5}'::jsonb, updated_at = now()
WHERE name = 'Free';

-- +goose Down
UPDATE plans
SET quotas = quotas - 'free_applications', updated_at = now()
WHERE name = 'Free';

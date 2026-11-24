-- +goose Up
-- Free plan per the product decision (study, section 4): no emails sent by 9itchi,
-- one CV version, one free CV download (with the 9itchi logo). Paid plans are created by the admin.
UPDATE plans
SET quotas = '{"emails_per_month": 0, "resumes": 1, "cv_downloads": 1}', updated_at = now()
WHERE name = 'Free';

-- +goose Down
UPDATE plans
SET quotas = '{"emails_per_month": 20, "resumes": 3}', updated_at = now()
WHERE name = 'Free';

-- +goose Up
-- There is no free offer any more: without a subscription a student writes and checks a CV, but cannot download
-- it and sees only a part of the diagnosis (contact and summary). "Free" stays the name of that state in the code.
-- cv_downloads is now counted per period of a subscription; ats_<group> = 0 hides a group of the diagnosis.
UPDATE plans
SET quotas = quotas || '{"cv_downloads": 0, "ats_experience": 0, "ats_education": 0, "ats_skills": 0, "ats_layout": 0, "ats_file": 0}'::jsonb,
    updated_at = now()
WHERE name = 'Free';

-- +goose Down
UPDATE plans
SET quotas = (quotas - 'ats_experience' - 'ats_education' - 'ats_skills' - 'ats_layout' - 'ats_file') || '{"cv_downloads": 1}'::jsonb,
    updated_at = now()
WHERE name = 'Free';

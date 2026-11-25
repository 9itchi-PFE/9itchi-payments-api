-- +goose Up
-- Decided 25 Nov. 2026: without a subscription a student browses the offers and sends 3 applications with the CV
-- they imported, as it is. No CV diagnosis (every group hidden), no AI, no download, and editing the CV in 9itchi
-- (cv_edit = 0) is for subscribers. Every value stays editable in the admin (Plans).
UPDATE plans
SET quotas = quotas || '{"free_applications": 3, "cv_edit": 0, "cv_downloads": 0, "ai_drafts_per_month": 0, "ai_rewrites_per_month": 0, "ai_matches_per_month": 0, "ats_contact": 0, "ats_summary": 0, "ats_experience": 0, "ats_education": 0, "ats_skills": 0, "ats_layout": 0, "ats_file": 0}'::jsonb,
    updated_at = now()
WHERE name = 'Free';

-- +goose Down
UPDATE plans
SET quotas = (quotas - 'cv_edit' - 'ats_contact' - 'ats_summary')
    || '{"free_applications": 5, "ai_drafts_per_month": 1, "ai_rewrites_per_month": 1, "ai_matches_per_month": 3}'::jsonb,
    updated_at = now()
WHERE name = 'Free';

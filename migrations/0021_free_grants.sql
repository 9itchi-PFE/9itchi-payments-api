-- +goose Up
-- Decided 28 Nov. 2026: without a subscription a student only reads the offers and the PFE books. No CV import, no
-- application, no AI, nothing else (every Free quota at 0, still editable in the admin Plans).
UPDATE plans
SET quotas = quotas || '{"resumes": 0, "free_applications": 0, "ai_scores_per_day": 0, "emails_per_month": 0, "cv_edit": 0, "cv_downloads": 0, "ai_drafts_per_month": 0, "ai_rewrites_per_month": 0, "ai_matches_per_month": 0, "recruiter_searches_per_day": 0}'::jsonb,
    details = '{
  "fr": {"label": "Découverte", "tagline": "Pour commencer ta recherche de PFE", "features": ["Offres de PFE vérifiées et PFE books", "Alertes sur les nouvelles offres"]},
  "en": {"label": "Discovery", "tagline": "To start your PFE search", "features": ["Reviewed PFE offers and PFE books", "Alerts on new offers"]},
  "ar": {"label": "اكتشاف", "tagline": "لتبدأ بحثك عن تربص ختم الدروس", "features": ["عروض PFE مُراجَعة وكتب PFE", "تنبيهات بالعروض الجديدة"]}
}'::jsonb,
    updated_at = now()
WHERE lower(name) = 'free';

-- Free usage the team gives to chosen emails: while the student has no subscription, these quotas replace the Free
-- ones key by key. Applies only to an email a Google or LinkedIn sign-in vouched for.
CREATE TABLE free_grants (
  email text PRIMARY KEY CHECK (email = lower(email) AND email <> ''),
  quotas jsonb NOT NULL DEFAULT '{}'::jsonb,
  note text NOT NULL DEFAULT '',
  given_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE free_grants;
UPDATE plans
SET quotas = quotas || '{"resumes": 1, "free_applications": 3, "ai_scores_per_day": 3}'::jsonb,
    details = '{
  "fr": {"label": "Découverte", "tagline": "Pour commencer ta recherche de PFE", "features": ["Offres de PFE vérifiées et PFE books", "3 candidatures avec ton CV importé", "Alertes sur les nouvelles offres"]},
  "en": {"label": "Discovery", "tagline": "To start your PFE search", "features": ["Reviewed PFE offers and PFE books", "3 applications with your imported CV", "Alerts on new offers"]},
  "ar": {"label": "اكتشاف", "tagline": "لتبدأ بحثك عن تربص ختم الدروس", "features": ["عروض PFE مُراجَعة وكتب PFE", "3 ترشحات بسيرتك الذاتية المستوردة", "تنبيهات بالعروض الجديدة"]}
}'::jsonb,
    updated_at = now()
WHERE lower(name) = 'free';

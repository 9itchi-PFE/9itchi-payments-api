-- +goose Up
-- What a student reads about a plan, per language (fr, en, ar): a label, a tagline and a list of features. The admin
-- writes it with the plan; an empty object means the client falls back to its own texts.
ALTER TABLE plans ADD COLUMN details jsonb NOT NULL DEFAULT '{}'::jsonb;

UPDATE plans
SET details = '{
  "fr": {"label": "Découverte", "tagline": "Pour commencer ta recherche de PFE", "features": ["Offres de PFE vérifiées et PFE books", "3 candidatures avec ton CV importé", "Alertes sur les nouvelles offres"]},
  "en": {"label": "Discovery", "tagline": "To start your PFE search", "features": ["Reviewed PFE offers and PFE books", "3 applications with your imported CV", "Alerts on new offers"]},
  "ar": {"label": "اكتشاف", "tagline": "لتبدأ بحثك عن تربص ختم الدروس", "features": ["عروض PFE مُراجَعة وكتب PFE", "3 ترشحات بسيرتك الذاتية المستوردة", "تنبيهات بالعروض الجديدة"]}
}'::jsonb,
    updated_at = now()
WHERE name = 'Free';

-- +goose Down
ALTER TABLE plans DROP COLUMN IF EXISTS details;

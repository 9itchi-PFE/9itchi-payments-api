-- +goose Up
CREATE TABLE payments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL,
  plan_id uuid NOT NULL REFERENCES plans (id),
  promo_code_id uuid REFERENCES promo_codes (id) ON DELETE SET NULL,
  provider text NOT NULL CHECK (length(provider) > 0),
  provider_ref text CHECK (provider_ref IS NULL OR length(provider_ref) > 0),
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'failed')),
  amount_millimes bigint NOT NULL CHECK (amount_millimes >= 0),
  discount_millimes bigint NOT NULL DEFAULT 0 CHECK (discount_millimes >= 0),
  completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_payments_provider_ref ON payments (provider, provider_ref);
CREATE INDEX idx_payments_user_status ON payments (user_id, status);
CREATE INDEX idx_payments_plan ON payments (plan_id);
CREATE INDEX idx_payments_promo ON payments (promo_code_id);

-- +goose Down
DROP TABLE payments;

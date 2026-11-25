-- +goose Up
-- Days the team gave a student for free: who gave them, on which subscription, and the period they added. A gift is
-- not a payment: it has no receipt and no amount.
CREATE TABLE subscription_gifts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL,
  subscription_id uuid NOT NULL REFERENCES subscriptions (id) ON DELETE CASCADE,
  plan_id uuid NOT NULL REFERENCES plans (id),
  days integer NOT NULL CHECK (days > 0),
  given_by uuid NOT NULL,
  period_start timestamptz NOT NULL,
  period_end timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (period_start < period_end)
);
CREATE INDEX idx_subscription_gifts_user ON subscription_gifts (user_id, created_at DESC, id DESC);
CREATE INDEX idx_subscription_gifts_subscription ON subscription_gifts (subscription_id);
CREATE INDEX idx_subscription_gifts_plan ON subscription_gifts (plan_id);

-- +goose Down
DROP TABLE subscription_gifts;

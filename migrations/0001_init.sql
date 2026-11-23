-- +goose Up
CREATE TABLE plans (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL UNIQUE CHECK (length(name) > 0),
  price numeric(12,2) NOT NULL DEFAULT 0 CHECK (price >= 0),
  duration_days integer NOT NULL CHECK (duration_days > 0),
  quotas jsonb NOT NULL DEFAULT '{}',
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_plans_active ON plans (active);

CREATE TABLE promo_codes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code text NOT NULL UNIQUE CHECK (length(code) > 0 AND code = upper(code)),
  type text NOT NULL CHECK (type IN ('percentage', 'fixed')),
  value numeric(12,2) NOT NULL CHECK (value > 0),
  valid_from timestamptz NOT NULL,
  valid_until timestamptz NOT NULL,
  usage_limit integer NOT NULL DEFAULT 0 CHECK (usage_limit >= 0),
  used_count integer NOT NULL DEFAULT 0 CHECK (used_count >= 0),
  plan_ids uuid[] NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (type <> 'percentage' OR value BETWEEN 1 AND 100),
  CHECK (valid_from < valid_until)
);

CREATE TABLE subscriptions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL,
  plan_id uuid NOT NULL REFERENCES plans (id),
  promo_code_id uuid REFERENCES promo_codes (id) ON DELETE SET NULL,
  status text NOT NULL CHECK (status IN ('pending', 'active', 'cancelled', 'expired')),
  amount numeric(12,2) NOT NULL DEFAULT 0 CHECK (amount >= 0),
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (starts_at < ends_at)
);
CREATE UNIQUE INDEX uq_subscriptions_user_active ON subscriptions (user_id) WHERE status = 'active';
CREATE INDEX idx_subscriptions_user_status ON subscriptions (user_id, status);
CREATE INDEX idx_subscriptions_plan ON subscriptions (plan_id);
CREATE INDEX idx_subscriptions_promo ON subscriptions (promo_code_id);

INSERT INTO plans (name, price, duration_days, quotas, active)
VALUES ('Free', 0, 36500, '{"emails_per_month": 20, "resumes": 3}', true);

-- +goose Down
DROP TABLE subscriptions;
DROP TABLE promo_codes;
DROP TABLE plans;

-- +goose Up
-- A sale is a reduction without a code, between two days (both included, counted in Tunis): every student sees
-- the lower price and the usual one struck through, and pays the lower price. plan_ids empty: every paid plan.
CREATE TABLE sales (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL CHECK (length(name) BETWEEN 1 AND 60),
  percent integer NOT NULL CHECK (percent BETWEEN 1 AND 90),
  starts_on date NOT NULL,
  ends_on date NOT NULL,
  plan_ids uuid[] NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (starts_on <= ends_on)
);
CREATE INDEX sales_days_idx ON sales (ends_on, starts_on);
-- The sale a payment was opened in, by its name that day, so a receipt keeps saying why the price was lower.
ALTER TABLE payments ADD COLUMN sale_name text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE payments DROP COLUMN sale_name;
DROP TABLE sales;

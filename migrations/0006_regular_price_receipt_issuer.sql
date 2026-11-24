-- +goose Up
-- The price a plan normally costs, shown struck through next to the price charged. NULL: nothing to strike.
ALTER TABLE plans ADD COLUMN regular_price numeric(12,2) CHECK (regular_price IS NULL OR regular_price > price);
-- The regular price as it was when the payment was opened, so a receipt keeps saying what was true that day.
ALTER TABLE payments ADD COLUMN regular_millimes bigint CHECK (regular_millimes IS NULL OR regular_millimes > 0);

-- Who issues the receipts. One row; a field stays empty until the admin fills it in.
CREATE TABLE receipt_issuer (
  id boolean PRIMARY KEY DEFAULT true CHECK (id),
  legal_name text NOT NULL DEFAULT '',
  address text NOT NULL DEFAULT '',
  tax_id text NOT NULL DEFAULT '',
  vat_mention text NOT NULL DEFAULT '',
  updated_by uuid,
  updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO receipt_issuer DEFAULT VALUES;

-- +goose Down
DROP TABLE receipt_issuer;
ALTER TABLE payments DROP COLUMN regular_millimes;
ALTER TABLE plans DROP COLUMN regular_price;

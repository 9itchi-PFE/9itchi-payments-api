-- +goose Up
-- A completed payment gets a receipt number and the period it paid for (shown on the receipt).
-- The sequence can leave gaps when a completing transaction rolls back.
CREATE SEQUENCE payment_receipt_seq;
ALTER TABLE payments
  ADD COLUMN receipt_no bigint,
  ADD COLUMN period_start timestamptz,
  ADD COLUMN period_end timestamptz,
  ADD CONSTRAINT payments_period_check CHECK (period_start IS NULL OR period_end IS NULL OR period_end > period_start);

UPDATE payments p SET receipt_no = n.rn
FROM (SELECT id, row_number() OVER (ORDER BY completed_at, id) AS rn FROM payments WHERE status = 'completed') n
WHERE p.id = n.id;
SELECT setval('payment_receipt_seq', COALESCE((SELECT max(receipt_no) FROM payments), 0) + 1, false);

CREATE UNIQUE INDEX uq_payments_receipt_no ON payments (receipt_no) WHERE receipt_no IS NOT NULL;
CREATE INDEX idx_payments_user_completed ON payments (user_id, completed_at DESC, id DESC) WHERE status = 'completed';

-- +goose Down
DROP INDEX idx_payments_user_completed;
DROP INDEX uq_payments_receipt_no;
ALTER TABLE payments
  DROP CONSTRAINT payments_period_check,
  DROP COLUMN period_end,
  DROP COLUMN period_start,
  DROP COLUMN receipt_no;
DROP SEQUENCE payment_receipt_seq;

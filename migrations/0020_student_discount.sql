-- +goose Up
-- Student discount: a percentage the admin sets, taken off at checkout for a user whose email is on one of the
-- student domains (or a subdomain of one) and was vouched for by a Google or LinkedIn sign-in. 0 = off.
CREATE TABLE student_discount (
  id boolean PRIMARY KEY DEFAULT true CHECK (id),
  percent integer NOT NULL DEFAULT 0 CHECK (percent BETWEEN 0 AND 100),
  domains text[] NOT NULL DEFAULT '{}',
  updated_by uuid,
  updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO student_discount DEFAULT VALUES;

-- The student percentage a payment was opened with (0: none), so the receipt can name it.
ALTER TABLE payments ADD COLUMN student_percent integer NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE payments DROP COLUMN student_percent;
DROP TABLE student_discount;

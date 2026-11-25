-- +goose Up
-- The launch price a student pays, kept for them: a renewal of the same plan costs it again as long as they
-- renew with no gap of more than 7 days, even once the launch price is over for everyone else. In millimes;
-- NULL: the student did not subscribe at a launch price.
ALTER TABLE subscriptions ADD COLUMN launch_millimes bigint CHECK (launch_millimes IS NULL OR launch_millimes > 0);
-- The launch price that applied to a payment (the one it charged, before a promo code), copied onto the
-- subscription it creates.
ALTER TABLE payments ADD COLUMN launch_millimes bigint CHECK (launch_millimes IS NULL OR launch_millimes > 0);

-- +goose Down
ALTER TABLE payments DROP COLUMN launch_millimes;
ALTER TABLE subscriptions DROP COLUMN launch_millimes;

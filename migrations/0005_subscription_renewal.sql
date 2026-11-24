-- +goose Up
-- A subscription renews every period unless the student stops it: stopped_at says when they did. Flouci cannot
-- charge by itself, so "renews" means the student is asked to pay again when the period ends; a stopped
-- subscription simply ends, with no payment asked. Access always lasts until ends_at.
ALTER TABLE subscriptions ADD COLUMN stopped_at timestamptz;
CREATE INDEX idx_subscriptions_user_ends ON subscriptions (user_id, ends_at DESC);

-- +goose Down
DROP INDEX idx_subscriptions_user_ends;
ALTER TABLE subscriptions DROP COLUMN stopped_at;

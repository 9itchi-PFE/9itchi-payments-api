-- +goose Up
-- The period (named by its end) a renewal reminder was already raised for: one a few days before the end, one after
-- the period ended without a renewal. A renewal moves ends_at, so the next period gets its own reminders.
ALTER TABLE subscriptions ADD COLUMN due_reminded_for timestamptz, ADD COLUMN lapse_reminded_for timestamptz;

-- +goose Down
ALTER TABLE subscriptions DROP COLUMN due_reminded_for, DROP COLUMN lapse_reminded_for;

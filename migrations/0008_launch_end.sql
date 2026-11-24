-- +goose Up
-- The last day of the launch price. From the next day (in Tunis) the plan costs its regular price and nothing is
-- struck through any more. NULL: the launch price has no end. It only makes sense with a regular price.
ALTER TABLE plans ADD COLUMN launch_ends_on date CHECK (launch_ends_on IS NULL OR regular_price IS NOT NULL);

-- +goose Down
ALTER TABLE plans DROP COLUMN launch_ends_on;

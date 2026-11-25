-- +goose Up
-- Plan names are unique without case ("Yearly" and "yearly" would show as the same plan to students), as the
-- admin screen already checks. A clash is a unique violation, answered 409 like any duplicate name.
CREATE UNIQUE INDEX uq_plans_name_lower ON plans (lower(name));

-- +goose Down
DROP INDEX IF EXISTS uq_plans_name_lower;

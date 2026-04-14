-- +goose Up
-- +goose StatementBegin
ALTER TABLE actions DROP CONSTRAINT IF EXISTS actions_kind_check;
ALTER TABLE actions ADD CONSTRAINT actions_kind_check CHECK (kind IN ('log','webhook','email','workflow'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE actions DROP CONSTRAINT IF EXISTS actions_kind_check;
ALTER TABLE actions ADD CONSTRAINT actions_kind_check CHECK (kind IN ('log','webhook','email'));
-- +goose StatementEnd

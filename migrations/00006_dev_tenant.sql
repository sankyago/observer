-- +goose Up
-- +goose StatementBegin
INSERT INTO tenants (id, slug, name)
VALUES ('00000000-0000-0000-0000-000000000001', 'dev', 'Dev Tenant')
ON CONFLICT (slug) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM tenants WHERE slug = 'dev';
-- +goose StatementEnd

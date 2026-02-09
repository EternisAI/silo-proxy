-- +goose Up
-- +goose StatementBegin
CREATE TYPE provisioning_key_status AS ENUM ('active', 'exhausted', 'expired', 'revoked');

CREATE TABLE IF NOT EXISTS provisioning_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status provisioning_key_status NOT NULL DEFAULT 'active',
    max_uses INT NOT NULL DEFAULT 1,
    used_count INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMP,
    notes TEXT
);

CREATE INDEX IF NOT EXISTS idx_provisioning_keys_user_id ON provisioning_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_provisioning_keys_status ON provisioning_keys(status);
CREATE INDEX IF NOT EXISTS idx_provisioning_keys_key_hash ON provisioning_keys(key_hash);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_provisioning_keys_key_hash;
DROP INDEX IF EXISTS idx_provisioning_keys_status;
DROP INDEX IF EXISTS idx_provisioning_keys_user_id;
DROP TABLE IF EXISTS provisioning_keys;
DROP TYPE IF EXISTS provisioning_key_status;
-- +goose StatementEnd

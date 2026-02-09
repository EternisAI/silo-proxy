-- +goose Up
-- +goose StatementBegin
CREATE TYPE agent_status AS ENUM ('active', 'inactive', 'suspended');

CREATE TABLE IF NOT EXISTS agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provisioned_with_key_id UUID REFERENCES provisioning_keys(id) ON DELETE SET NULL,
    status agent_status NOT NULL DEFAULT 'active',
    cert_fingerprint VARCHAR(255),
    registered_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_ip_address INET,
    metadata JSONB,
    notes TEXT
);

CREATE INDEX IF NOT EXISTS idx_agents_user_id ON agents(user_id);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_last_seen_at ON agents(last_seen_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_agents_last_seen_at;
DROP INDEX IF EXISTS idx_agents_status;
DROP INDEX IF EXISTS idx_agents_user_id;
DROP TABLE IF EXISTS agents;
DROP TYPE IF EXISTS agent_status;
-- +goose StatementEnd

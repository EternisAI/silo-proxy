-- +goose Up
CREATE TABLE agent_certificates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id VARCHAR(255) NOT NULL UNIQUE,
    serial_number VARCHAR(255) NOT NULL UNIQUE,
    subject_common_name VARCHAR(255) NOT NULL,
    not_before TIMESTAMP NOT NULL,
    not_after TIMESTAMP NOT NULL,
    cert_pem TEXT NOT NULL,
    key_pem TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    revoked_at TIMESTAMP,
    revoked_reason VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agent_certificates_user_id ON agent_certificates(user_id);
CREATE INDEX idx_agent_certificates_agent_id ON agent_certificates(agent_id);
CREATE INDEX idx_agent_certificates_serial_number ON agent_certificates(serial_number);
CREATE INDEX idx_agent_certificates_is_active ON agent_certificates(is_active);
CREATE INDEX idx_agent_certificates_not_after ON agent_certificates(not_after);

-- +goose Down
DROP TABLE IF EXISTS agent_certificates;

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS agent_connection_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    connected_at TIMESTAMP NOT NULL DEFAULT NOW(),
    disconnected_at TIMESTAMP,
    duration_seconds INT,
    ip_address INET,
    disconnect_reason VARCHAR(255)
);

CREATE INDEX IF NOT EXISTS idx_connection_logs_agent_id ON agent_connection_logs(agent_id);
CREATE INDEX IF NOT EXISTS idx_connection_logs_connected_at ON agent_connection_logs(connected_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_connection_logs_connected_at;
DROP INDEX IF EXISTS idx_connection_logs_agent_id;
DROP TABLE IF EXISTS agent_connection_logs;
-- +goose StatementEnd

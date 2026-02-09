-- name: CreateConnectionLog :one
INSERT INTO agent_connection_logs (agent_id, connected_at, ip_address)
VALUES ($1, $2, $3) RETURNING *;

-- name: UpdateConnectionLog :exec
UPDATE agent_connection_logs
SET disconnected_at = $2,
    duration_seconds = EXTRACT(EPOCH FROM ($2 - connected_at))::INT,
    disconnect_reason = $3
WHERE id = $1;

-- name: GetAgentConnectionHistory :many
SELECT * FROM agent_connection_logs
WHERE agent_id = $1
ORDER BY connected_at DESC
LIMIT $2 OFFSET $3;

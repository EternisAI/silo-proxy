-- name: CreateAgent :one
INSERT INTO agents (user_id, provisioned_with_key_id, metadata, notes)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetAgentByID :one
SELECT * FROM agents WHERE id = $1 LIMIT 1;

-- name: ListAgentsByUser :many
SELECT * FROM agents WHERE user_id = $1 ORDER BY registered_at DESC;

-- name: UpdateAgentLastSeen :exec
UPDATE agents SET last_seen_at = $2, last_ip_address = $3 WHERE id = $1;

-- name: UpdateAgentStatus :exec
UPDATE agents SET status = $2 WHERE id = $1;

-- name: UpdateAgentCertFingerprint :exec
UPDATE agents SET cert_fingerprint = $2 WHERE id = $1;

-- name: CreateProvisioningKey :one
INSERT INTO provisioning_keys (key_hash, user_id, max_uses, expires_at, notes)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: GetProvisioningKeyByHash :one
SELECT * FROM provisioning_keys
WHERE key_hash = $1 AND status = 'active' LIMIT 1;

-- name: ListProvisioningKeysByUser :many
SELECT * FROM provisioning_keys WHERE user_id = $1 ORDER BY created_at DESC;

-- name: IncrementKeyUsage :exec
UPDATE provisioning_keys
SET used_count = used_count + 1,
    status = CASE WHEN used_count + 1 >= max_uses
                  THEN 'exhausted'::provisioning_key_status
                  ELSE status END,
    updated_at = NOW()
WHERE id = $1;

-- name: RevokeProvisioningKey :exec
UPDATE provisioning_keys
SET status = 'revoked'::provisioning_key_status, revoked_at = NOW(), updated_at = NOW()
WHERE id = $1 AND user_id = $2;

-- name: ExpireOldKeys :exec
UPDATE provisioning_keys
SET status = 'expired'::provisioning_key_status, updated_at = NOW()
WHERE status = 'active' AND expires_at < NOW();

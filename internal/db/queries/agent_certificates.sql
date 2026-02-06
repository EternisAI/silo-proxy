-- name: CreateAgentCertificate :one
INSERT INTO agent_certificates (
    user_id,
    agent_id,
    serial_number,
    subject_common_name,
    not_before,
    not_after,
    cert_pem,
    key_pem
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: GetCertificateByAgentID :one
SELECT * FROM agent_certificates
WHERE agent_id = $1
LIMIT 1;

-- name: GetCertificateBySerial :one
SELECT * FROM agent_certificates
WHERE serial_number = $1
LIMIT 1;

-- name: GetCertificateByID :one
SELECT * FROM agent_certificates
WHERE id = $1
LIMIT 1;

-- name: ListCertificatesByUser :many
SELECT * FROM agent_certificates
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: ListAllCertificates :many
SELECT * FROM agent_certificates
ORDER BY created_at DESC;

-- name: RevokeCertificate :one
UPDATE agent_certificates
SET revoked_at = NOW(),
    revoked_reason = $2,
    is_active = false
WHERE agent_id = $1
RETURNING *;

-- name: DeleteCertificate :exec
DELETE FROM agent_certificates
WHERE agent_id = $1;

-- name: CheckCertificateValid :one
SELECT id, agent_id, user_id, is_active, revoked_at, not_after
FROM agent_certificates
WHERE agent_id = $1
  AND is_active = true
  AND revoked_at IS NULL
  AND not_after > NOW()
LIMIT 1;

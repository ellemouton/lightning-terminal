-- name: InsertSession :one
INSERT INTO sessions (
    legacy_id, label, session_state, session_type, expiry, created_at,
    revoked_at, server_address, dev_server, macaroon_root_key, pairing_secret,
    local_private_key, local_public_key, remote_public_key, privacy, group_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11, $12,
    $13, $14, $15, $16
) RETURNING id;

-- name: SetSessionGroupID :exec
UPDATE sessions
SET group_id = $1
WHERE id = $2;

-- name: GetSessionByLocalPublicKey :one
SELECT * FROM sessions
WHERE local_public_key = $1;

-- name: GetSessionsInGroup :many
SELECT * FROM sessions
WHERE group_id = $1;

-- name: GetSessionLegacyIDsInGroup :many
SELECT legacy_id FROM sessions
WHERE group_id = $1;

-- name: GetSessionByID :one
SELECT * FROM sessions
WHERE id = $1;

-- name: GetSessionIDByLegacyID :one
SELECT id FROM sessions
WHERE legacy_id = $1;

-- name: GetLegacyIDBySessionID :one
SELECT legacy_id FROM sessions
WHERE id = $1;

-- name: GetSessionByLegacyID :one
SELECT * FROM sessions
WHERE legacy_id = $1;

-- name: ListSessions :many
SELECT * FROM sessions;

-- name: ListSessionsByType :many
SELECT * FROM sessions
WHERE session_type = $1;

-- name: UpdateSessionState :exec
UPDATE sessions
SET session_state = $1
WHERE id = $2;

-- name: SetRemotePublicKeyByLocalPublicKey :exec
UPDATE sessions
SET remote_public_key = $1
WHERE local_public_key = $2;

-- name: InsertMacaroonPermission :exec
INSERT INTO macaroon_permissions (
    session_id, entity, action
) VALUES (
    $1, $2, $3
);

-- name: GetSessionMacaroonPermissions :many
SELECT * FROM macaroon_permissions
WHERE session_id = $1;

-- name: InsertMacaroonCaveat :exec
INSERT INTO macaroon_caveats (
    session_id, id, verification_id, location
) VALUES (
$1, $2, $3, $4
);

-- name: GetSessionMacaroonCaveats :many
SELECT * FROM macaroon_caveats
WHERE session_id = $1;

-- name: InsertFeatureConfig :exec
INSERT INTO feature_configs (
    session_id, feature_name, config
) VALUES (
    $1, $2, $3
);

-- name: GetSessionFeatureConfigs :many
SELECT * FROM feature_configs
WHERE session_id = $1;

-- name: InsertPrivacyFlag :exec
INSERT INTO privacy_flags (
    session_id, flag
) VALUES (
    $1, $2
);

-- name: GetSessionPrivacyFlags :many
SELECT * FROM privacy_flags
WHERE session_id = $1;
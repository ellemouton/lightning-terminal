-- name: InsertKVStoreRecord :exec
INSERT INTO kvstores (perm, rule_name, session_id, feature_name, key, value)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetKVStoreRecord :one
SELECT value
FROM kvstores
WHERE key = sqlc.arg('key')
  AND rule_name = sqlc.arg('rule_name')
  AND perm = sqlc.arg('perm')
  AND (session_id = sqlc.narg('session_id') OR sqlc.narg('session_id') IS NULL)
  AND (feature_name = sqlc.narg('feature_name') OR sqlc.narg('feature_name') IS NULL);

-- name: DeleteKVStoreRecord :exec
DELETE FROM kvstores
WHERE key = sqlc.arg('key')
  AND rule_name = sqlc.arg('rule_name')
  AND perm = sqlc.arg('perm')
  AND (session_id = sqlc.narg('session_id') OR sqlc.narg('session_id') IS NULL)
  AND (feature_name = sqlc.narg('feature_name') OR sqlc.narg('feature_name') IS NULL);

-- name: DeleteAllTemp :exec
DELETE FROM kvstores
WHERE perm = false;

-- name: UpdateKVStoreRecord :exec
UPDATE kvstores
SET value = $1
WHERE key = sqlc.arg('key')
  AND rule_name = sqlc.arg('rule_name')
  AND perm = sqlc.arg('perm')
  AND (session_id = sqlc.narg('session_id') OR sqlc.narg('session_id') IS NULL)
  AND (feature_name = sqlc.narg('feature_name') OR sqlc.narg('feature_name') IS NULL);

-- name: GetOrInsertRuleID :one
INSERT INTO rules (name)
VALUES ($1)
ON CONFLICT(name) DO UPDATE SET name = excluded.name
RETURNING id;

-- name: GetRuleID :one
SELECT id
FROM rules
WHERE name = sqlc.arg('name');

-- name: GetOrInsertFeatureID :one
INSERT INTO features (name)
VALUES ($1)
ON CONFLICT(name) DO UPDATE SET name = excluded.name
RETURNING id;

-- name: GetFeatureID :one
SELECT id
FROM features
WHERE name = sqlc.arg('name');

-- name: InsertKVStoreRecord :exec
INSERT INTO kvstores (perm, rule_id, session_id, feature_id, entry_key, value)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: DeleteAllTempKVStores :exec
DELETE FROM kvstores
WHERE perm = false;

-- name: GetGlobalKVStoreRecord :one
SELECT value
FROM kvstores
WHERE entry_key = sqlc.arg('key')
  AND rule_id = sqlc.arg('rule_id')
  AND perm = sqlc.arg('perm')
  AND session_id IS NULL
  AND feature_id IS NULL;

-- name: GetSessionKVStoreRecord :one
SELECT value
FROM kvstores
WHERE entry_key = sqlc.arg('key')
  AND rule_id = sqlc.arg('rule_id')
  AND perm = sqlc.arg('perm')
  AND session_id = sqlc.arg('session_id')
  AND feature_id IS NULL;

-- name: GetFeatureKVStoreRecord :one
SELECT value
FROM kvstores
WHERE entry_key = sqlc.arg('key')
  AND rule_id = sqlc.arg('rule_id')
  AND perm = sqlc.arg('perm')
  AND session_id = sqlc.arg('session_id')
  AND feature_id = sqlc.arg('feature_id');

-- name: DeleteGlobalKVStoreRecord :exec
DELETE FROM kvstores
WHERE entry_key = sqlc.arg('key')
  AND rule_id = sqlc.arg('rule_id')
  AND perm = sqlc.arg('perm')
  AND session_id IS NULL
  AND feature_id IS NULL;

-- name: DeleteSessionKVStoreRecord :exec
DELETE FROM kvstores
WHERE entry_key = sqlc.arg('key')
  AND rule_id = sqlc.arg('rule_id')
  AND perm = sqlc.arg('perm')
  AND session_id = sqlc.arg('session_id')
  AND feature_id IS NULL;

-- name: DeleteFeatureKVStoreRecord :exec
DELETE FROM kvstores
WHERE entry_key = sqlc.arg('key')
  AND rule_id = sqlc.arg('rule_id')
  AND perm = sqlc.arg('perm')
  AND session_id = sqlc.arg('session_id')
  AND feature_id = sqlc.arg('feature_id');

-- name: UpdateGlobalKVStoreRecord :exec
UPDATE kvstores
SET value = $1
WHERE entry_key = sqlc.arg('key')
  AND rule_id = sqlc.arg('rule_id')
  AND perm = sqlc.arg('perm')
  AND session_id IS NULL
  AND feature_id IS NULL;

-- name: UpdateSessionKVStoreRecord :exec
UPDATE kvstores
SET value = $1
WHERE entry_key = sqlc.arg('key')
  AND rule_id = sqlc.arg('rule_id')
  AND perm = sqlc.arg('perm')
  AND session_id = sqlc.arg('session_id')
  AND feature_id IS NULL;

-- name: UpdateFeatureKVStoreRecord :exec
UPDATE kvstores
SET value = $1
WHERE entry_key = sqlc.arg('key')
  AND rule_id = sqlc.arg('rule_id')
  AND perm = sqlc.arg('perm')
  AND session_id = sqlc.arg('session_id')
  AND feature_id = sqlc.arg('feature_id');

-- name: InsertAction :one
INSERT INTO actions (
    session_id, actor_name, feature_name, trigger, intent,
    structured_json_data, rpc_method, rpc_params_json, created_at,
    state, error_reason
) VALUES (
             $1, $2, $3, $4, $5, $6,
             $7, $8, $9, $10, $11
) RETURNING id;

-- name: SetActionState :exec
UPDATE actions
SET state = $1,
    error_reason = $2
WHERE id = $3;

-- name: ListActionsPaginated :many
SELECT a.*
FROM actions a
WHERE (a.session_id = sqlc.narg('session_id') OR sqlc.narg('session_id') IS NULL)
  AND (a.feature_name = sqlc.narg('feature_name') OR sqlc.narg('feature_name') IS NULL)
  AND (a.actor_name = sqlc.narg('actor_name') OR sqlc.narg('actor_name') IS NULL)
  AND (a.rpc_method = sqlc.narg('rpc_method') OR sqlc.narg('rpc_method') IS NULL)
  AND (a.state = sqlc.narg('state') OR sqlc.narg('state') IS NULL)
  AND (a.created_at <= sqlc.narg('end_time') OR sqlc.narg('end_time') IS NULL)
  AND (a.created_at >= sqlc.narg('start_time') OR sqlc.narg('start_time') IS NULL)
  AND (
    sqlc.narg('group_id')::BIGINT IS NULL OR EXISTS (
        SELECT 1
        FROM sessions s
        WHERE s.id = a.session_id AND s.group_id = sqlc.narg('group_id')::BIGINT
    )
    )
ORDER BY
    CASE WHEN sqlc.arg('reversed')::BOOLEAN THEN a.created_at END DESC,
    CASE WHEN NOT sqlc.arg('reversed')::BOOLEAN THEN a.created_at END ASC
    LIMIT sqlc.narg('limit')
OFFSET $1;

-- name: ListActions :many
SELECT a.*
FROM actions a
WHERE (a.session_id = sqlc.narg('session_id') OR sqlc.narg('session_id') IS NULL)
  AND (a.feature_name = sqlc.narg('feature_name') OR sqlc.narg('feature_name') IS NULL)
  AND (a.actor_name = sqlc.narg('actor_name') OR sqlc.narg('actor_name') IS NULL)
  AND (a.rpc_method = sqlc.narg('rpc_method') OR sqlc.narg('rpc_method') IS NULL)
  AND (a.state = sqlc.narg('state') OR sqlc.narg('state') IS NULL)
  AND (a.created_at <= sqlc.narg('end_time') OR sqlc.narg('end_time') IS NULL)
  AND (a.created_at >= sqlc.narg('start_time') OR sqlc.narg('start_time') IS NULL)
  AND (
    sqlc.narg('group_id')::BIGINT IS NULL OR EXISTS (
        SELECT 1
        FROM sessions s
        WHERE s.id = a.session_id AND s.group_id = sqlc.narg('group_id')::BIGINT
    )
    )
ORDER BY
    CASE WHEN sqlc.arg('reversed')::BOOLEAN THEN a.created_at END DESC,
    CASE WHEN NOT sqlc.arg('reversed')::BOOLEAN THEN a.created_at END ASC;


-- name: CountActions :one
SELECT COUNT(*)
FROM actions a
WHERE (a.session_id = sqlc.narg('session_id') OR sqlc.narg('session_id') IS NULL)
  AND (a.feature_name = sqlc.narg('feature_name') OR sqlc.narg('feature_name') IS NULL)
  AND (a.actor_name = sqlc.narg('actor_name') OR sqlc.narg('actor_name') IS NULL)
  AND (a.rpc_method = sqlc.narg('rpc_method') OR sqlc.narg('rpc_method') IS NULL)
  AND (a.state = sqlc.narg('state') OR sqlc.narg('state') IS NULL)
  AND (a.created_at <= sqlc.narg('end_time') OR sqlc.narg('end_time') IS NULL)
  AND (a.created_at >= sqlc.narg('start_time') OR sqlc.narg('start_time') IS NULL)
  AND (
    sqlc.narg('group_id')::BIGINT IS NULL OR EXISTS (
        SELECT 1
        FROM sessions s
        WHERE s.id = a.session_id AND s.group_id = sqlc.narg('group_id')::BIGINT
));
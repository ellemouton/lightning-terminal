// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.25.0
// source: actions.sql

package sqlc

import (
	"context"
	"database/sql"
	"time"
)

const countActions = `-- name: CountActions :one
SELECT COUNT(*)
FROM actions a
WHERE (a.session_id = $1 OR $1 IS NULL)
  AND (a.account_id = $2 OR $2 IS NULL)
  AND (a.feature_name = $3 OR $3 IS NULL)
  AND (a.actor_name = $4 OR $4 IS NULL)
  AND (a.rpc_method = $5 OR $5 IS NULL)
  AND (a.state = $6 OR $6 IS NULL)
  AND (a.created_at <= $7 OR $7 IS NULL)
  AND (a.created_at >= $8 OR $8 IS NULL)
  AND (
    $9::BIGINT IS NULL OR EXISTS (
        SELECT 1
        FROM sessions s
        WHERE s.id = a.session_id AND s.group_id = $9::BIGINT
))
`

type CountActionsParams struct {
	SessionID   sql.NullInt64
	AccountID   sql.NullInt64
	FeatureName sql.NullString
	ActorName   sql.NullString
	RpcMethod   sql.NullString
	State       sql.NullInt16
	EndTime     sql.NullTime
	StartTime   sql.NullTime
	GroupID     sql.NullInt64
}

func (q *Queries) CountActions(ctx context.Context, arg CountActionsParams) (int64, error) {
	row := q.db.QueryRowContext(ctx, countActions,
		arg.SessionID,
		arg.AccountID,
		arg.FeatureName,
		arg.ActorName,
		arg.RpcMethod,
		arg.State,
		arg.EndTime,
		arg.StartTime,
		arg.GroupID,
	)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const insertAction = `-- name: InsertAction :one
INSERT INTO actions (
    session_id, account_id, macaroon_identifier, actor_name, feature_name, trigger, intent,
    structured_json_data, rpc_method, rpc_params_json, created_at,
    state, error_reason
) VALUES (
             $1, $2, $3, $4, $5, $6,
             $7, $8, $9, $10, $11, $12, $13
) RETURNING id
`

type InsertActionParams struct {
	SessionID          sql.NullInt64
	AccountID          sql.NullInt64
	MacaroonIdentifier []byte
	ActorName          sql.NullString
	FeatureName        sql.NullString
	Trigger            sql.NullString
	Intent             sql.NullString
	StructuredJsonData sql.NullString
	RpcMethod          string
	RpcParamsJson      []byte
	CreatedAt          time.Time
	State              int16
	ErrorReason        sql.NullString
}

func (q *Queries) InsertAction(ctx context.Context, arg InsertActionParams) (int64, error) {
	row := q.db.QueryRowContext(ctx, insertAction,
		arg.SessionID,
		arg.AccountID,
		arg.MacaroonIdentifier,
		arg.ActorName,
		arg.FeatureName,
		arg.Trigger,
		arg.Intent,
		arg.StructuredJsonData,
		arg.RpcMethod,
		arg.RpcParamsJson,
		arg.CreatedAt,
		arg.State,
		arg.ErrorReason,
	)
	var id int64
	err := row.Scan(&id)
	return id, err
}

const listActions = `-- name: ListActions :many
SELECT a.id, a.session_id, a.account_id, a.macaroon_identifier, a.actor_name, a.feature_name, a.trigger, a.intent, a.structured_json_data, a.rpc_method, a.rpc_params_json, a.created_at, a.state, a.error_reason
FROM actions a
WHERE (a.session_id = $1 OR $1 IS NULL)
  AND (a.account_id = $2 OR $2 IS NULL)
  AND (a.feature_name = $3 OR $3 IS NULL)
  AND (a.actor_name = $4 OR $4 IS NULL)
  AND (a.rpc_method = $5 OR $5 IS NULL)
  AND (a.state = $6 OR $6 IS NULL)
  AND (a.created_at <= $7 OR $7 IS NULL)
  AND (a.created_at >= $8 OR $8 IS NULL)
  AND (
    $9::BIGINT IS NULL OR EXISTS (
        SELECT 1
        FROM sessions s
        WHERE s.id = a.session_id AND s.group_id = $9::BIGINT
    )
    )
ORDER BY
    CASE WHEN $10::BOOLEAN THEN a.created_at END DESC,
    CASE WHEN NOT $10::BOOLEAN THEN a.created_at END ASC
`

type ListActionsParams struct {
	SessionID   sql.NullInt64
	AccountID   sql.NullInt64
	FeatureName sql.NullString
	ActorName   sql.NullString
	RpcMethod   sql.NullString
	State       sql.NullInt16
	EndTime     sql.NullTime
	StartTime   sql.NullTime
	GroupID     sql.NullInt64
	Reversed    bool
}

func (q *Queries) ListActions(ctx context.Context, arg ListActionsParams) ([]Action, error) {
	rows, err := q.db.QueryContext(ctx, listActions,
		arg.SessionID,
		arg.AccountID,
		arg.FeatureName,
		arg.ActorName,
		arg.RpcMethod,
		arg.State,
		arg.EndTime,
		arg.StartTime,
		arg.GroupID,
		arg.Reversed,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Action
	for rows.Next() {
		var i Action
		if err := rows.Scan(
			&i.ID,
			&i.SessionID,
			&i.AccountID,
			&i.MacaroonIdentifier,
			&i.ActorName,
			&i.FeatureName,
			&i.Trigger,
			&i.Intent,
			&i.StructuredJsonData,
			&i.RpcMethod,
			&i.RpcParamsJson,
			&i.CreatedAt,
			&i.State,
			&i.ErrorReason,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listActionsPaginated = `-- name: ListActionsPaginated :many
SELECT a.id, a.session_id, a.account_id, a.macaroon_identifier, a.actor_name, a.feature_name, a.trigger, a.intent, a.structured_json_data, a.rpc_method, a.rpc_params_json, a.created_at, a.state, a.error_reason
FROM actions a
WHERE (a.session_id = $2 OR $2 IS NULL)
  AND (a.account_id = $3 OR $3 IS NULL)
  AND (a.feature_name = $4 OR $4 IS NULL)
  AND (a.actor_name = $5 OR $5 IS NULL)
  AND (a.rpc_method = $6 OR $6 IS NULL)
  AND (a.state = $7 OR $7 IS NULL)
  AND (a.created_at <= $8 OR $8 IS NULL)
  AND (a.created_at >= $9 OR $9 IS NULL)
  AND (
    $10::BIGINT IS NULL OR EXISTS (
        SELECT 1
        FROM sessions s
        WHERE s.id = a.session_id AND s.group_id = $10::BIGINT
    )
    )
ORDER BY
    CASE WHEN $11::BOOLEAN THEN a.created_at END DESC,
    CASE WHEN NOT $11::BOOLEAN THEN a.created_at END ASC
    LIMIT $12
OFFSET $1
`

type ListActionsPaginatedParams struct {
	Offset      int32
	SessionID   sql.NullInt64
	AccountID   sql.NullInt64
	FeatureName sql.NullString
	ActorName   sql.NullString
	RpcMethod   sql.NullString
	State       sql.NullInt16
	EndTime     sql.NullTime
	StartTime   sql.NullTime
	GroupID     sql.NullInt64
	Reversed    bool
	Limit       sql.NullInt32
}

func (q *Queries) ListActionsPaginated(ctx context.Context, arg ListActionsPaginatedParams) ([]Action, error) {
	rows, err := q.db.QueryContext(ctx, listActionsPaginated,
		arg.Offset,
		arg.SessionID,
		arg.AccountID,
		arg.FeatureName,
		arg.ActorName,
		arg.RpcMethod,
		arg.State,
		arg.EndTime,
		arg.StartTime,
		arg.GroupID,
		arg.Reversed,
		arg.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Action
	for rows.Next() {
		var i Action
		if err := rows.Scan(
			&i.ID,
			&i.SessionID,
			&i.AccountID,
			&i.MacaroonIdentifier,
			&i.ActorName,
			&i.FeatureName,
			&i.Trigger,
			&i.Intent,
			&i.StructuredJsonData,
			&i.RpcMethod,
			&i.RpcParamsJson,
			&i.CreatedAt,
			&i.State,
			&i.ErrorReason,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const setActionState = `-- name: SetActionState :exec
UPDATE actions
SET state = $1,
    error_reason = $2
WHERE id = $3
`

type SetActionStateParams struct {
	State       int16
	ErrorReason sql.NullString
	ID          int64
}

func (q *Queries) SetActionState(ctx context.Context, arg SetActionStateParams) error {
	_, err := q.db.ExecContext(ctx, setActionState, arg.State, arg.ErrorReason, arg.ID)
	return err
}

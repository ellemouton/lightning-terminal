package sqlc

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
)

type ActionQueryParams struct {
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

func buildActionsQuery(params ActionQueryParams, count bool) (string, []interface{}) {
	var (
		conditions []string
		args       []interface{}
	)

	if params.SessionID.Valid {
		conditions = append(conditions, "a.session_id = ?")
		args = append(args, params.SessionID.Int64)
	}
	if params.AccountID.Valid {
		conditions = append(conditions, "a.account_id = ?")
		args = append(args, params.AccountID.Int64)
	}
	if params.FeatureName.Valid {
		conditions = append(conditions, "a.feature_name = ?")
		args = append(args, params.FeatureName.String)
	}
	if params.ActorName.Valid {
		conditions = append(conditions, "a.actor_name = ?")
		args = append(args, params.ActorName.String)
	}
	if params.RpcMethod.Valid {
		conditions = append(conditions, "a.rpc_method = ?")
		args = append(args, params.RpcMethod.String)
	}
	if params.State.Valid {
		conditions = append(conditions, "a.action_state = ?")
		args = append(args, params.State.Int16)
	}
	if params.EndTime.Valid {
		conditions = append(conditions, "a.created_at <= ?")
		args = append(args, params.EndTime.Time)
	}
	if params.StartTime.Valid {
		conditions = append(conditions, "a.created_at >= ?")
		args = append(args, params.StartTime.Time)
	}
	if params.GroupID.Valid {
		conditions = append(conditions, `
			EXISTS (
				SELECT 1
				FROM sessions s
				WHERE s.id = a.session_id AND s.group_id = ?
			)`)
		args = append(args, params.GroupID.Int64)
	}

	query := "SELECT a.* FROM actions a"
	if count {
		query = "SELECT COUNT(*) FROM actions a"
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	return query, args
}

type ListActionsParams struct {
	ActionQueryParams
	Reversed interface{}
	*Pagination
}

type Pagination struct {
	NumOffset int32
	NumLimit  int32
}

func buildListActionsQuery(params ListActionsParams) (string, []interface{}) {
	query, args := buildActionsQuery(params.ActionQueryParams, false)

	// Determine order direction
	order := "ASC"
	if reversed, ok := params.Reversed.(bool); ok && reversed {
		order = "DESC"
	}
	query += " ORDER BY a.created_at " + order

	// Maybe paginate.
	if params.Pagination != nil {
		query += " LIMIT ? OFFSET ?"
		args = append(args, params.NumLimit, params.NumOffset)
	}

	return query, args
}

func (q *Queries) ListActions(ctx context.Context,
	arg ListActionsParams) ([]Action, error) {

	query, args := buildListActionsQuery(arg)
	rows, err := q.db.QueryContext(ctx, fillPlaceHolders(query), args...)
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
			&i.ActionTrigger,
			&i.Intent,
			&i.StructuredJsonData,
			&i.RpcMethod,
			&i.RpcParamsJson,
			&i.CreatedAt,
			&i.ActionState,
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

func (q *Queries) CountActions(ctx context.Context,
	arg ActionQueryParams) (int64, error) {

	query, args := buildActionsQuery(arg, true)
	row := q.db.QueryRowContext(ctx, query, args...)

	var count int64
	err := row.Scan(&count)

	return count, err
}

func fillPlaceHolders(query string) string {
	var sb strings.Builder
	argNum := 1

	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			sb.WriteString("$")
			sb.WriteString(itoa(argNum))
			argNum++
		} else {
			sb.WriteByte(query[i])
		}
	}

	return sb.String()
}

// Fast int-to-string helper for small ints (avoids strconv for performance)
func itoa(n int) string {
	if n < 10 {
		return string('0' + n)
	}
	return strconv.Itoa(n)
}

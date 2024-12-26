// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.25.0
// source: privacy_pairs.sql

package sqlc

import (
	"context"
)

const getAllPrivacyPairs = `-- name: GetAllPrivacyPairs :many
SELECT real, pseudo
FROM privacy_pairs
WHERE group_id = $1
`

type GetAllPrivacyPairsRow struct {
	Real   string
	Pseudo string
}

func (q *Queries) GetAllPrivacyPairs(ctx context.Context, groupID int64) ([]GetAllPrivacyPairsRow, error) {
	rows, err := q.db.QueryContext(ctx, getAllPrivacyPairs, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetAllPrivacyPairsRow
	for rows.Next() {
		var i GetAllPrivacyPairsRow
		if err := rows.Scan(&i.Real, &i.Pseudo); err != nil {
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

const getPseudoForReal = `-- name: GetPseudoForReal :one
SELECT pseudo
FROM privacy_pairs
WHERE group_id = $1 AND real = $2
`

type GetPseudoForRealParams struct {
	GroupID int64
	Real    string
}

func (q *Queries) GetPseudoForReal(ctx context.Context, arg GetPseudoForRealParams) (string, error) {
	row := q.db.QueryRowContext(ctx, getPseudoForReal, arg.GroupID, arg.Real)
	var pseudo string
	err := row.Scan(&pseudo)
	return pseudo, err
}

const getRealForPseudo = `-- name: GetRealForPseudo :one
SELECT real
FROM privacy_pairs
WHERE group_id = $1 AND pseudo = $2
`

type GetRealForPseudoParams struct {
	GroupID int64
	Pseudo  string
}

func (q *Queries) GetRealForPseudo(ctx context.Context, arg GetRealForPseudoParams) (string, error) {
	row := q.db.QueryRowContext(ctx, getRealForPseudo, arg.GroupID, arg.Pseudo)
	var real string
	err := row.Scan(&real)
	return real, err
}

const insertPrivacyPair = `-- name: InsertPrivacyPair :exec
INSERT INTO privacy_pairs (group_id, real, pseudo)
VALUES ($1, $2, $3)
`

type InsertPrivacyPairParams struct {
	GroupID int64
	Real    string
	Pseudo  string
}

func (q *Queries) InsertPrivacyPair(ctx context.Context, arg InsertPrivacyPairParams) error {
	_, err := q.db.ExecContext(ctx, insertPrivacyPair, arg.GroupID, arg.Real, arg.Pseudo)
	return err
}
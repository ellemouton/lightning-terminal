// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.25.0
// source: sessions.sql

package sqlc

import (
	"context"
	"database/sql"
	"time"
)

const deleteSessionsWithState = `-- name: DeleteSessionsWithState :exec
DELETE FROM sessions
WHERE state = $1
`

func (q *Queries) DeleteSessionsWithState(ctx context.Context, state int16) error {
	_, err := q.db.ExecContext(ctx, deleteSessionsWithState, state)
	return err
}

const getLegacyIDBySessionID = `-- name: GetLegacyIDBySessionID :one
SELECT legacy_id FROM sessions
WHERE id = $1
`

func (q *Queries) GetLegacyIDBySessionID(ctx context.Context, id int64) ([]byte, error) {
	row := q.db.QueryRowContext(ctx, getLegacyIDBySessionID, id)
	var legacy_id []byte
	err := row.Scan(&legacy_id)
	return legacy_id, err
}

const getSessionByID = `-- name: GetSessionByID :one
SELECT id, legacy_id, label, state, type, expiry, created_at, revoked_at, server_address, dev_server, macaroon_root_key, pairing_secret, local_private_key, local_public_key, remote_public_key, privacy, account_id, group_id FROM sessions
WHERE id = $1
`

func (q *Queries) GetSessionByID(ctx context.Context, id int64) (Session, error) {
	row := q.db.QueryRowContext(ctx, getSessionByID, id)
	var i Session
	err := row.Scan(
		&i.ID,
		&i.LegacyID,
		&i.Label,
		&i.State,
		&i.Type,
		&i.Expiry,
		&i.CreatedAt,
		&i.RevokedAt,
		&i.ServerAddress,
		&i.DevServer,
		&i.MacaroonRootKey,
		&i.PairingSecret,
		&i.LocalPrivateKey,
		&i.LocalPublicKey,
		&i.RemotePublicKey,
		&i.Privacy,
		&i.AccountID,
		&i.GroupID,
	)
	return i, err
}

const getSessionByLegacyID = `-- name: GetSessionByLegacyID :one
SELECT id, legacy_id, label, state, type, expiry, created_at, revoked_at, server_address, dev_server, macaroon_root_key, pairing_secret, local_private_key, local_public_key, remote_public_key, privacy, account_id, group_id FROM sessions
WHERE legacy_id = $1
`

func (q *Queries) GetSessionByLegacyID(ctx context.Context, legacyID []byte) (Session, error) {
	row := q.db.QueryRowContext(ctx, getSessionByLegacyID, legacyID)
	var i Session
	err := row.Scan(
		&i.ID,
		&i.LegacyID,
		&i.Label,
		&i.State,
		&i.Type,
		&i.Expiry,
		&i.CreatedAt,
		&i.RevokedAt,
		&i.ServerAddress,
		&i.DevServer,
		&i.MacaroonRootKey,
		&i.PairingSecret,
		&i.LocalPrivateKey,
		&i.LocalPublicKey,
		&i.RemotePublicKey,
		&i.Privacy,
		&i.AccountID,
		&i.GroupID,
	)
	return i, err
}

const getSessionByLocalPublicKey = `-- name: GetSessionByLocalPublicKey :one
SELECT id, legacy_id, label, state, type, expiry, created_at, revoked_at, server_address, dev_server, macaroon_root_key, pairing_secret, local_private_key, local_public_key, remote_public_key, privacy, account_id, group_id FROM sessions
WHERE local_public_key = $1
`

func (q *Queries) GetSessionByLocalPublicKey(ctx context.Context, localPublicKey []byte) (Session, error) {
	row := q.db.QueryRowContext(ctx, getSessionByLocalPublicKey, localPublicKey)
	var i Session
	err := row.Scan(
		&i.ID,
		&i.LegacyID,
		&i.Label,
		&i.State,
		&i.Type,
		&i.Expiry,
		&i.CreatedAt,
		&i.RevokedAt,
		&i.ServerAddress,
		&i.DevServer,
		&i.MacaroonRootKey,
		&i.PairingSecret,
		&i.LocalPrivateKey,
		&i.LocalPublicKey,
		&i.RemotePublicKey,
		&i.Privacy,
		&i.AccountID,
		&i.GroupID,
	)
	return i, err
}

const getSessionFeatureConfigs = `-- name: GetSessionFeatureConfigs :many
SELECT session_id, feature_name, config FROM feature_configs
WHERE session_id = $1
`

func (q *Queries) GetSessionFeatureConfigs(ctx context.Context, sessionID int64) ([]FeatureConfig, error) {
	rows, err := q.db.QueryContext(ctx, getSessionFeatureConfigs, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []FeatureConfig
	for rows.Next() {
		var i FeatureConfig
		if err := rows.Scan(&i.SessionID, &i.FeatureName, &i.Config); err != nil {
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

const getSessionIDByLegacyID = `-- name: GetSessionIDByLegacyID :one
SELECT id FROM sessions
WHERE legacy_id = $1
`

func (q *Queries) GetSessionIDByLegacyID(ctx context.Context, legacyID []byte) (int64, error) {
	row := q.db.QueryRowContext(ctx, getSessionIDByLegacyID, legacyID)
	var id int64
	err := row.Scan(&id)
	return id, err
}

const getSessionLegacyIDsInGroup = `-- name: GetSessionLegacyIDsInGroup :many
SELECT legacy_id FROM sessions
WHERE group_id = $1
`

func (q *Queries) GetSessionLegacyIDsInGroup(ctx context.Context, groupID sql.NullInt64) ([][]byte, error) {
	rows, err := q.db.QueryContext(ctx, getSessionLegacyIDsInGroup, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items [][]byte
	for rows.Next() {
		var legacy_id []byte
		if err := rows.Scan(&legacy_id); err != nil {
			return nil, err
		}
		items = append(items, legacy_id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getSessionMacaroonCaveats = `-- name: GetSessionMacaroonCaveats :many
SELECT session_id, id, verification_id, location FROM macaroon_caveats
WHERE session_id = $1
`

func (q *Queries) GetSessionMacaroonCaveats(ctx context.Context, sessionID int64) ([]MacaroonCaveat, error) {
	rows, err := q.db.QueryContext(ctx, getSessionMacaroonCaveats, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []MacaroonCaveat
	for rows.Next() {
		var i MacaroonCaveat
		if err := rows.Scan(
			&i.SessionID,
			&i.ID,
			&i.VerificationID,
			&i.Location,
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

const getSessionMacaroonPermissions = `-- name: GetSessionMacaroonPermissions :many
SELECT session_id, entity, action FROM macaroon_permissions
WHERE session_id = $1
`

func (q *Queries) GetSessionMacaroonPermissions(ctx context.Context, sessionID int64) ([]MacaroonPermission, error) {
	rows, err := q.db.QueryContext(ctx, getSessionMacaroonPermissions, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []MacaroonPermission
	for rows.Next() {
		var i MacaroonPermission
		if err := rows.Scan(&i.SessionID, &i.Entity, &i.Action); err != nil {
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

const getSessionPrivacyFlags = `-- name: GetSessionPrivacyFlags :many
SELECT session_id, flag FROM privacy_flags
WHERE session_id = $1
`

func (q *Queries) GetSessionPrivacyFlags(ctx context.Context, sessionID int64) ([]PrivacyFlag, error) {
	rows, err := q.db.QueryContext(ctx, getSessionPrivacyFlags, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PrivacyFlag
	for rows.Next() {
		var i PrivacyFlag
		if err := rows.Scan(&i.SessionID, &i.Flag); err != nil {
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

const getSessionsInGroup = `-- name: GetSessionsInGroup :many
SELECT id, legacy_id, label, state, type, expiry, created_at, revoked_at, server_address, dev_server, macaroon_root_key, pairing_secret, local_private_key, local_public_key, remote_public_key, privacy, account_id, group_id FROM sessions
WHERE group_id = $1
`

func (q *Queries) GetSessionsInGroup(ctx context.Context, groupID sql.NullInt64) ([]Session, error) {
	rows, err := q.db.QueryContext(ctx, getSessionsInGroup, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Session
	for rows.Next() {
		var i Session
		if err := rows.Scan(
			&i.ID,
			&i.LegacyID,
			&i.Label,
			&i.State,
			&i.Type,
			&i.Expiry,
			&i.CreatedAt,
			&i.RevokedAt,
			&i.ServerAddress,
			&i.DevServer,
			&i.MacaroonRootKey,
			&i.PairingSecret,
			&i.LocalPrivateKey,
			&i.LocalPublicKey,
			&i.RemotePublicKey,
			&i.Privacy,
			&i.AccountID,
			&i.GroupID,
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

const insertFeatureConfig = `-- name: InsertFeatureConfig :exec
INSERT INTO feature_configs (
    session_id, feature_name, config
) VALUES (
    $1, $2, $3
)
`

type InsertFeatureConfigParams struct {
	SessionID   int64
	FeatureName string
	Config      []byte
}

func (q *Queries) InsertFeatureConfig(ctx context.Context, arg InsertFeatureConfigParams) error {
	_, err := q.db.ExecContext(ctx, insertFeatureConfig, arg.SessionID, arg.FeatureName, arg.Config)
	return err
}

const insertMacaroonCaveat = `-- name: InsertMacaroonCaveat :exec
INSERT INTO macaroon_caveats (
    session_id, id, verification_id, location
) VALUES (
    $1, $2, $3, $4
)
`

type InsertMacaroonCaveatParams struct {
	SessionID      int64
	ID             []byte
	VerificationID []byte
	Location       sql.NullString
}

func (q *Queries) InsertMacaroonCaveat(ctx context.Context, arg InsertMacaroonCaveatParams) error {
	_, err := q.db.ExecContext(ctx, insertMacaroonCaveat,
		arg.SessionID,
		arg.ID,
		arg.VerificationID,
		arg.Location,
	)
	return err
}

const insertMacaroonPermission = `-- name: InsertMacaroonPermission :exec
INSERT INTO macaroon_permissions (
    session_id, entity, action
) VALUES (
    $1, $2, $3
)
`

type InsertMacaroonPermissionParams struct {
	SessionID int64
	Entity    string
	Action    string
}

func (q *Queries) InsertMacaroonPermission(ctx context.Context, arg InsertMacaroonPermissionParams) error {
	_, err := q.db.ExecContext(ctx, insertMacaroonPermission, arg.SessionID, arg.Entity, arg.Action)
	return err
}

const insertPrivacyFlag = `-- name: InsertPrivacyFlag :exec
INSERT INTO privacy_flags (
    session_id, flag
) VALUES (
    $1, $2
)
`

type InsertPrivacyFlagParams struct {
	SessionID int64
	Flag      int32
}

func (q *Queries) InsertPrivacyFlag(ctx context.Context, arg InsertPrivacyFlagParams) error {
	_, err := q.db.ExecContext(ctx, insertPrivacyFlag, arg.SessionID, arg.Flag)
	return err
}

const insertSession = `-- name: InsertSession :one
INSERT INTO sessions (
    legacy_id, label, state, type, expiry, created_at,
    server_address, dev_server, macaroon_root_key, pairing_secret,
    local_private_key, local_public_key, remote_public_key, privacy, group_id, account_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11, $12,
    $13, $14, $15, $16
) RETURNING id
`

type InsertSessionParams struct {
	LegacyID        []byte
	Label           string
	State           int16
	Type            int16
	Expiry          time.Time
	CreatedAt       time.Time
	ServerAddress   string
	DevServer       bool
	MacaroonRootKey int64
	PairingSecret   []byte
	LocalPrivateKey []byte
	LocalPublicKey  []byte
	RemotePublicKey []byte
	Privacy         bool
	GroupID         sql.NullInt64
	AccountID       sql.NullInt64
}

func (q *Queries) InsertSession(ctx context.Context, arg InsertSessionParams) (int64, error) {
	row := q.db.QueryRowContext(ctx, insertSession,
		arg.LegacyID,
		arg.Label,
		arg.State,
		arg.Type,
		arg.Expiry,
		arg.CreatedAt,
		arg.ServerAddress,
		arg.DevServer,
		arg.MacaroonRootKey,
		arg.PairingSecret,
		arg.LocalPrivateKey,
		arg.LocalPublicKey,
		arg.RemotePublicKey,
		arg.Privacy,
		arg.GroupID,
		arg.AccountID,
	)
	var id int64
	err := row.Scan(&id)
	return id, err
}

const listSessions = `-- name: ListSessions :many
SELECT id, legacy_id, label, state, type, expiry, created_at, revoked_at, server_address, dev_server, macaroon_root_key, pairing_secret, local_private_key, local_public_key, remote_public_key, privacy, account_id, group_id FROM sessions
ORDER BY created_at
`

func (q *Queries) ListSessions(ctx context.Context) ([]Session, error) {
	rows, err := q.db.QueryContext(ctx, listSessions)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Session
	for rows.Next() {
		var i Session
		if err := rows.Scan(
			&i.ID,
			&i.LegacyID,
			&i.Label,
			&i.State,
			&i.Type,
			&i.Expiry,
			&i.CreatedAt,
			&i.RevokedAt,
			&i.ServerAddress,
			&i.DevServer,
			&i.MacaroonRootKey,
			&i.PairingSecret,
			&i.LocalPrivateKey,
			&i.LocalPublicKey,
			&i.RemotePublicKey,
			&i.Privacy,
			&i.AccountID,
			&i.GroupID,
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

const listSessionsByState = `-- name: ListSessionsByState :many
SELECT id, legacy_id, label, state, type, expiry, created_at, revoked_at, server_address, dev_server, macaroon_root_key, pairing_secret, local_private_key, local_public_key, remote_public_key, privacy, account_id, group_id FROM sessions
WHERE state = $1
ORDER BY created_at
`

func (q *Queries) ListSessionsByState(ctx context.Context, state int16) ([]Session, error) {
	rows, err := q.db.QueryContext(ctx, listSessionsByState, state)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Session
	for rows.Next() {
		var i Session
		if err := rows.Scan(
			&i.ID,
			&i.LegacyID,
			&i.Label,
			&i.State,
			&i.Type,
			&i.Expiry,
			&i.CreatedAt,
			&i.RevokedAt,
			&i.ServerAddress,
			&i.DevServer,
			&i.MacaroonRootKey,
			&i.PairingSecret,
			&i.LocalPrivateKey,
			&i.LocalPublicKey,
			&i.RemotePublicKey,
			&i.Privacy,
			&i.AccountID,
			&i.GroupID,
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

const listSessionsByType = `-- name: ListSessionsByType :many
SELECT id, legacy_id, label, state, type, expiry, created_at, revoked_at, server_address, dev_server, macaroon_root_key, pairing_secret, local_private_key, local_public_key, remote_public_key, privacy, account_id, group_id FROM sessions
WHERE type = $1
ORDER BY created_at
`

func (q *Queries) ListSessionsByType(ctx context.Context, type_ int16) ([]Session, error) {
	rows, err := q.db.QueryContext(ctx, listSessionsByType, type_)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Session
	for rows.Next() {
		var i Session
		if err := rows.Scan(
			&i.ID,
			&i.LegacyID,
			&i.Label,
			&i.State,
			&i.Type,
			&i.Expiry,
			&i.CreatedAt,
			&i.RevokedAt,
			&i.ServerAddress,
			&i.DevServer,
			&i.MacaroonRootKey,
			&i.PairingSecret,
			&i.LocalPrivateKey,
			&i.LocalPublicKey,
			&i.RemotePublicKey,
			&i.Privacy,
			&i.AccountID,
			&i.GroupID,
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

const setRemotePublicKeyByLocalPublicKey = `-- name: SetRemotePublicKeyByLocalPublicKey :exec
UPDATE sessions
SET remote_public_key = $1
WHERE local_public_key = $2
`

type SetRemotePublicKeyByLocalPublicKeyParams struct {
	RemotePublicKey []byte
	LocalPublicKey  []byte
}

func (q *Queries) SetRemotePublicKeyByLocalPublicKey(ctx context.Context, arg SetRemotePublicKeyByLocalPublicKeyParams) error {
	_, err := q.db.ExecContext(ctx, setRemotePublicKeyByLocalPublicKey, arg.RemotePublicKey, arg.LocalPublicKey)
	return err
}

const setSessionGroupID = `-- name: SetSessionGroupID :exec
UPDATE sessions
SET group_id = $1
WHERE id = $2
`

type SetSessionGroupIDParams struct {
	GroupID sql.NullInt64
	ID      int64
}

func (q *Queries) SetSessionGroupID(ctx context.Context, arg SetSessionGroupIDParams) error {
	_, err := q.db.ExecContext(ctx, setSessionGroupID, arg.GroupID, arg.ID)
	return err
}

const setSessionRevokedAt = `-- name: SetSessionRevokedAt :exec
UPDATE sessions
SET revoked_at = $1
WHERE id = $2
`

type SetSessionRevokedAtParams struct {
	RevokedAt sql.NullTime
	ID        int64
}

func (q *Queries) SetSessionRevokedAt(ctx context.Context, arg SetSessionRevokedAtParams) error {
	_, err := q.db.ExecContext(ctx, setSessionRevokedAt, arg.RevokedAt, arg.ID)
	return err
}

const updateSessionState = `-- name: UpdateSessionState :exec
UPDATE sessions
SET state = $1
WHERE id = $2
`

type UpdateSessionStateParams struct {
	State int16
	ID    int64
}

func (q *Queries) UpdateSessionState(ctx context.Context, arg UpdateSessionStateParams) error {
	_, err := q.db.ExecContext(ctx, updateSessionState, arg.State, arg.ID)
	return err
}

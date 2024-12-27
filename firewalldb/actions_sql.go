package firewalldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lightninglabs/lightning-terminal/db"
	"github.com/lightninglabs/lightning-terminal/db/sqlc"
	"github.com/lightninglabs/lightning-terminal/session"
)

// SQLSessionQueries is a subset of the sqlc.Queries interface that can be used
// to interact with the sessions tables.
type SQLSessionQueries interface {
	GetSessionIDByLegacyID(ctx context.Context, legacyID []byte) (int64, error)
	GetLegacyIDBySessionID(ctx context.Context, id int64) ([]byte, error)
}

// SQLActionQueries is a subset of the sqlc.Queries interface that can be used
// to interact with action related tables.
type SQLActionQueries interface {
	SQLSessionQueries

	InsertAction(ctx context.Context, arg sqlc.InsertActionParams) (int64, error)
	SetActionState(ctx context.Context, arg sqlc.SetActionStateParams) error
	ListActions(ctx context.Context, arg sqlc.ListActionsParams) ([]sqlc.Action, error)
	ListActionsPaginated(ctx context.Context, arg sqlc.ListActionsPaginatedParams) ([]sqlc.Action, error)
	CountActions(ctx context.Context, arg sqlc.CountActionsParams) (int64, error)
}

// SQLQueriesTxOptions defines the set of db txn options the SQLQueries
// understands.
type SQLQueriesTxOptions struct {
	// readOnly governs if a read only transaction is needed or not.
	readOnly bool
}

// ReadOnly returns true if the transaction should be read only.
//
// NOTE: This implements the TxOptions.
func (a *SQLQueriesTxOptions) ReadOnly() bool {
	return a.readOnly
}

// NewSQLQueryReadTx creates a new read transaction option set.
func NewSQLQueryReadTx() SQLQueriesTxOptions {
	return SQLQueriesTxOptions{
		readOnly: true,
	}
}

// BatchedSQLActionsQueries is a version of the SQLActionQueries that's capable
// of batched database operations.
type BatchedSQLActionsQueries interface {
	SQLActionQueries

	db.BatchedTx[SQLActionQueries]
}

// SQLActionsStore represents a storage backend.
type SQLActionsStore struct {
	db BatchedSQLActionsQueries
}

var _ ActionDB = (*SQLActionsStore)(nil)

type sqlActionLocator struct {
	id int64
}

func (s *sqlActionLocator) isActionLocator() {}

var _ ActionLocator = (*sqlActionLocator)(nil)

// NewSQLActionsStore creates a new SQLActionsStore instance given an open
// BatchedSQLActionsQueries storage backend.
func NewSQLActionsStore(db BatchedSQLActionsQueries) *SQLActionsStore {
	return &SQLActionsStore{
		db: db,
	}
}

// GetActionsReadDB is a method on DB that constructs an ActionsReadDB.
//
// NOTE: This is part of the ActionDB interface.
func (s *SQLActionsStore) GetActionsReadDB(groupID session.ID,
	featureName string) ActionsReadDB {

	return &allActionsReadDB{
		db:          s,
		groupID:     groupID,
		featureName: featureName,
	}
}

// AddAction persists the given action to the database.
//
// NOTE: This is a part of the ActionDB interface.
func (s *SQLActionsStore) AddAction(ctx context.Context, a *Action) (
	ActionLocator, error) {

	var (
		writeTxOpts SQLQueriesTxOptions
		locator     sqlActionLocator

		actor          = sql.NullString{String: a.ActorName}
		feature        = sql.NullString{String: a.FeatureName}
		trigger        = sql.NullString{String: a.Trigger}
		intent         = sql.NullString{String: a.Intent}
		structuredJson = sql.NullString{String: a.StructuredJsonData}
		errReason      = sql.NullString{String: a.ErrorReason}
	)
	if a.ActorName != "" {
		actor.Valid = true
	}
	if a.FeatureName != "" {
		feature.Valid = true
	}
	if a.Trigger != "" {
		trigger.Valid = true
	}
	if a.Intent != "" {
		intent.Valid = true
	}
	if a.StructuredJsonData != "" {
		structuredJson.Valid = true
	}
	if a.ErrorReason != "" {
		errReason.Valid = true
	}

	err := s.db.ExecTx(ctx, &writeTxOpts, func(db SQLActionQueries) error {
		// The Action struct carries around the legacy session ID.
		// We first need to convert this to the DB ID used in the
		// sessions table if it is not an empty ID.
		// TODO(elle): OR must be from accounts table!!
		var sessionID sql.NullInt64
		if a.SessionID != session.EmptyID {
			id, err := db.GetSessionIDByLegacyID(
				ctx, a.SessionID[:],
			)
			if errors.Is(err, sql.ErrNoRows) {
				return session.ErrSessionUnknown
			} else if err != nil {
				return fmt.Errorf("unable to get DB ID for "+
					"legacy session ID %x: %w", a.SessionID,
					err)
			}

			sessionID = sql.NullInt64{
				Int64: id,
				Valid: true,
			}
		}

		id, err := db.InsertAction(ctx, sqlc.InsertActionParams{
			SessionID:          sessionID,
			ActorName:          actor,
			FeatureName:        feature,
			Trigger:            trigger,
			Intent:             intent,
			StructuredJsonData: structuredJson,
			RpcMethod:          a.RPCMethod,
			RpcParamsJson:      a.RPCParamsJson,
			CreatedAt:          a.AttemptedAt,
			State:              int16(a.State),
			ErrorReason:        errReason,
		})
		if err != nil {
			return err
		}

		locator = sqlActionLocator{
			id: id,
		}

		return nil
	}, func() {})
	if err != nil {
		return nil, err
	}

	return &locator, nil
}

// SetActionState finds the action specified by the ActionLocator and sets its
// state to the given state.
//
// NOTE: This is a part of the ActionDB interface.
func (s *SQLActionsStore) SetActionState(ctx context.Context, al ActionLocator,
	state ActionState, errReason string) error {

	if errReason != "" && state != ActionStateError {
		return fmt.Errorf("error reason should only be set for " +
			"ActionStateError")
	}

	locator, ok := al.(*sqlActionLocator)
	if !ok {
		return fmt.Errorf("expected sqlActionLocator, got %T", al)
	}

	var writeTxOpts SQLQueriesTxOptions
	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLActionQueries) error {
		return db.SetActionState(ctx, sqlc.SetActionStateParams{
			ID:    locator.id,
			State: int16(state),
			ErrorReason: sql.NullString{
				String: errReason,
				Valid:  errReason != "",
			},
		})
	}, func() {})
}

// ListActions returns a list of Actions. The query IndexOffset and MaxNum
// params can be used to control the number of actions returned.
// ListActionOptions may be used to filter on specific Action values. The return
// values are the list of actions, the last index and the total count (iff
// query.CountTotal is set).
//
// NOTE: This is part of the ActionDB interface.
func (s *SQLActionsStore) ListActions(ctx context.Context,
	query *ListActionsQuery, options ...ListActionOption) ([]*Action,
	uint64, uint64, error) {

	opts := newListActionOptions()
	for _, o := range options {
		o(opts)
	}

	var (
		readTxOpts = NewSQLQueryReadTx()
		actions    []*Action
		lastIndex  uint64
		totalCount int64
	)
	err := s.db.ExecTx(ctx, &readTxOpts, func(db SQLActionQueries) error {
		var (
			actorName   = sql.NullString{String: opts.actorName}
			feature     = sql.NullString{String: opts.featureName}
			rpcMethod   = sql.NullString{String: opts.methodName}
			actionState = sql.NullInt16{Int16: int16(opts.state)}
			startTime   = sql.NullTime{Time: opts.startTime}
			endTime     = sql.NullTime{Time: opts.endTime}
		)
		if opts.actorName != "" {
			actorName.Valid = true
		}
		if opts.featureName != "" {
			feature.Valid = true
		}
		if opts.methodName != "" {
			rpcMethod.Valid = true
		}
		if opts.state != 0 {
			actionState.Valid = true
		}
		if !opts.startTime.IsZero() {
			startTime.Valid = true
		}
		if !opts.endTime.IsZero() {
			endTime.Valid = true
		}

		var sessionID sql.NullInt64
		if opts.sessionID != session.EmptyID {
			sID, err := db.GetSessionIDByLegacyID(ctx, opts.sessionID[:])
			if errors.Is(err, sql.ErrNoRows) {
				return session.ErrSessionUnknown
			} else if err != nil {
				return fmt.Errorf("unable to get DB ID for legacy "+
					"session ID %x: %w", opts.sessionID, err)
			}

			sessionID = sql.NullInt64{
				Int64: sID,
				Valid: true,
			}
		}

		var groupID sql.NullInt64
		if opts.groupID != session.EmptyID {
			gID, err := db.GetSessionIDByLegacyID(ctx, opts.groupID[:])
			if errors.Is(err, sql.ErrNoRows) {
				return session.ErrUnknownGroup
			} else if err != nil {
				return fmt.Errorf("unable to get DB ID for legacy "+
					"group ID %x: %w", opts.groupID, err)
			}

			groupID = sql.NullInt64{
				Int64: gID,
				Valid: true,
			}
		}

		var (
			dbActions []sqlc.Action
			err       error
		)
		if query == nil {
			dbActions, err = db.ListActions(
				ctx, sqlc.ListActionsParams{
					SessionID:   sessionID,
					GroupID:     groupID,
					FeatureName: feature,
					ActorName:   actorName,
					RpcMethod:   rpcMethod,
					State:       actionState,
					EndTime:     endTime,
					StartTime:   startTime,
				},
			)
		} else {
			var limit sql.NullInt32
			if query.MaxNum != 0 {
				limit = sql.NullInt32{
					Int32: int32(query.MaxNum),
					Valid: true,
				}
			}
			dbActions, err = db.ListActionsPaginated(
				ctx, sqlc.ListActionsPaginatedParams{
					Limit:       limit,
					Offset:      int32(query.IndexOffset),
					Reversed:    query.Reversed,
					SessionID:   sessionID,
					GroupID:     groupID,
					FeatureName: feature,
					ActorName:   actorName,
					RpcMethod:   rpcMethod,
					State:       actionState,
					EndTime:     endTime,
					StartTime:   startTime,
				},
			)
		}
		if err != nil {
			return fmt.Errorf("unable to list actions: %w", err)
		}

		if query != nil && query.CountAll {
			totalCount, err = db.CountActions(
				ctx, sqlc.CountActionsParams{
					SessionID:   sessionID,
					GroupID:     groupID,
					FeatureName: feature,
					ActorName:   actorName,
					RpcMethod:   rpcMethod,
					State:       actionState,
					EndTime:     endTime,
					StartTime:   startTime,
				},
			)
			if err != nil {
				return fmt.Errorf("unable to count actions: %w",
					err)
			}
		}

		actions = make([]*Action, len(dbActions))
		for i, dbAction := range dbActions {
			action, err := unmarshalAction(ctx, db, dbAction)
			if err != nil {
				return fmt.Errorf("unable to unmarshal "+
					"action: %w", err)
			}

			actions[i] = action
			lastIndex = uint64(dbAction.ID)
		}

		return nil
	}, func() {
		actions = nil
	})

	return actions, lastIndex, uint64(totalCount), err
}

func unmarshalAction(ctx context.Context, db SQLSessionQueries,
	dbAction sqlc.Action) (*Action, error) {

	var legacySessID session.ID
	if dbAction.SessionID.Valid {
		legacySessIDB, err := db.GetLegacyIDBySessionID(
			ctx, dbAction.SessionID.Int64,
		)
		if err != nil {
			return nil, fmt.Errorf("unable to get legacy "+
				"session ID for session ID %d: %w",
				dbAction.SessionID.Int64, err)
		}

		legacySessID, err = session.IDFromBytes(legacySessIDB)
		if err != nil {
			return nil, err
		}
	}

	return &Action{
		SessionID:          legacySessID,
		ActorName:          dbAction.ActorName.String,
		FeatureName:        dbAction.FeatureName.String,
		Trigger:            dbAction.Trigger.String,
		Intent:             dbAction.Intent.String,
		StructuredJsonData: dbAction.StructuredJsonData.String,
		RPCMethod:          dbAction.RpcMethod,
		RPCParamsJson:      dbAction.RpcParamsJson,
		AttemptedAt:        dbAction.CreatedAt,
		State:              ActionState(dbAction.State),
		ErrorReason:        dbAction.ErrorReason.String,
	}, nil
}

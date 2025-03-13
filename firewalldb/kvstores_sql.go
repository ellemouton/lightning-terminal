package firewalldb

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lightninglabs/lightning-terminal/db"
	"github.com/lightninglabs/lightning-terminal/db/sqlc"
	"github.com/lightninglabs/lightning-terminal/session"
	"github.com/lightningnetwork/lnd/fn"
)

// SQLSessionQueries is a subset of the sqlc.Queries interface that can be used
// to interact with the sessions tables.
type SQLSessionQueries interface {
	GetSessionIDByAlias(ctx context.Context, legacyID []byte) (int64, error)
}

type SQLKVStoreQueries interface {
	SQLSessionQueries

	DeleteFeatureKVStoreRecord(ctx context.Context, arg sqlc.DeleteFeatureKVStoreRecordParams) error
	DeleteGlobalKVStoreRecord(ctx context.Context, arg sqlc.DeleteGlobalKVStoreRecordParams) error
	DeleteSessionKVStoreRecord(ctx context.Context, arg sqlc.DeleteSessionKVStoreRecordParams) error
	GetFeatureKVStoreRecord(ctx context.Context, arg sqlc.GetFeatureKVStoreRecordParams) ([]byte, error)
	GetGlobalKVStoreRecord(ctx context.Context, arg sqlc.GetGlobalKVStoreRecordParams) ([]byte, error)
	GetSessionKVStoreRecord(ctx context.Context, arg sqlc.GetSessionKVStoreRecordParams) ([]byte, error)
	UpdateFeatureKVStoreRecord(ctx context.Context, arg sqlc.UpdateFeatureKVStoreRecordParams) error
	UpdateGlobalKVStoreRecord(ctx context.Context, arg sqlc.UpdateGlobalKVStoreRecordParams) error
	UpdateSessionKVStoreRecord(ctx context.Context, arg sqlc.UpdateSessionKVStoreRecordParams) error
	InsertKVStoreRecord(ctx context.Context, arg sqlc.InsertKVStoreRecordParams) error
	DeleteAllTempKVStores(ctx context.Context) error
	GetOrInsertFeatureID(ctx context.Context, name string) (int64, error)
	GetOrInsertRuleID(ctx context.Context, name string) (int64, error)
	GetFeatureID(ctx context.Context, name string) (int64, error)
	GetRuleID(ctx context.Context, name string) (int64, error)
}

func (s *SQLDB) DeleteTempKVStores(ctx context.Context) error {
	var writeTxOpts db.QueriesTxOptions

	return s.db.ExecTx(ctx, &writeTxOpts, func(tx SQLQueries) error {
		return tx.DeleteAllTempKVStores(ctx)
	})
}

func (s *SQLDB) GetKVStores(rule string, groupAlias session.ID,
	feature string) KVStores {

	return &sqlExecutor[KVStoreTx]{
		db: s.db,
		wrapTx: func(queries SQLQueries) KVStoreTx {
			return &sqlKVStoresSQLTx{
				queries: queries,
				db: &kvStoreSQLDB{
					SQLDB:      s,
					groupAlias: groupAlias,
					rule:       rule,
					feature:    feature,
				},
			}
		},
	}
}

type kvStoreSQLDB struct {
	*SQLDB
	groupAlias session.ID
	rule       string
	feature    string
}

type sqlKVStoresSQLTx struct {
	db      *kvStoreSQLDB
	queries SQLKVStoreQueries
}

type sqlKVStore struct {
	*sqlKVStoresSQLTx

	params *sqlKVStoreParams
}

func (s *sqlKVStoresSQLTx) Global() KVStore {
	return &sqlKVStore{
		sqlKVStoresSQLTx: s,
		params: &sqlKVStoreParams{
			perm:     true,
			ruleName: s.db.rule,
		},
	}
}

func (s *sqlKVStoresSQLTx) Local() KVStore {
	var featureName fn.Option[string]
	if s.db.feature != "" {
		featureName = fn.Some(s.db.feature)
	}

	return &sqlKVStore{
		sqlKVStoresSQLTx: s,
		params: &sqlKVStoreParams{
			perm:        true,
			ruleName:    s.db.rule,
			sessionID:   fn.Some(s.db.groupAlias),
			featureName: featureName,
		},
	}
}

func (s *sqlKVStoresSQLTx) GlobalTemp() KVStore {
	return &sqlKVStore{
		sqlKVStoresSQLTx: s,
		params: &sqlKVStoreParams{
			perm:     false,
			ruleName: s.db.rule,
		},
	}
}

func (s *sqlKVStoresSQLTx) LocalTemp() KVStore {
	var featureName fn.Option[string]
	if s.db.feature != "" {
		featureName = fn.Some(s.db.feature)
	}

	return &sqlKVStore{
		sqlKVStoresSQLTx: s,
		params: &sqlKVStoreParams{
			perm:        false,
			ruleName:    s.db.rule,
			sessionID:   fn.Some(s.db.groupAlias),
			featureName: featureName,
		},
	}
}

var _ KVStoreTx = (*sqlKVStoresSQLTx)(nil)

type sqlKVStoreParams struct {
	perm        bool
	ruleName    string
	sessionID   fn.Option[session.ID]
	featureName fn.Option[string]
}

func (s *sqlKVStore) genNamespaceFields(ctx context.Context,
	readOnly bool) (int64, sql.NullInt64, sql.NullInt64, error) {

	var (
		sessionID sql.NullInt64
		featureID sql.NullInt64
		ruleID    int64
		err       error
	)

	if readOnly {
		ruleID, err = s.queries.GetRuleID(ctx, s.params.ruleName)
		if err != nil {
			return 0, sessionID, featureID,
				fmt.Errorf("unable to get rule ID: %w", err)
		}
	} else {
		ruleID, err = s.queries.GetOrInsertRuleID(
			ctx, s.params.ruleName,
		)
		if err != nil {
			return 0, sessionID, featureID,
				fmt.Errorf("unable to get rule ID: %w", err)
		}
	}

	s.params.featureName.WhenSome(func(feature string) {
		var id int64
		if readOnly {
			id, err = s.queries.GetFeatureID(ctx, feature)
			if err != nil {
				return
			}
		} else {
			id, err = s.queries.GetOrInsertFeatureID(ctx, feature)
			if err != nil {
				return
			}
		}

		featureID = sql.NullInt64{
			Int64: id,
			Valid: true,
		}
	})
	if err != nil {
		return ruleID, sessionID, featureID, err
	}

	s.params.sessionID.WhenSome(func(id session.ID) {
		var groupID int64
		groupID, err = s.queries.GetSessionIDByAlias(ctx, id[:])
		if err != nil {
			return
		}

		sessionID = sql.NullInt64{
			Int64: groupID,
			Valid: true,
		}
	})
	return ruleID, sessionID, featureID, err
}

func (s *sqlKVStore) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := s.get(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return value, nil
}

func (s *sqlKVStore) get(ctx context.Context, key string) ([]byte, error) {
	ruleID, sessionID, featureID, err := s.genNamespaceFields(ctx, true)
	if err != nil {
		return nil, err
	}

	switch {
	case sessionID.Valid && featureID.Valid:
		return s.queries.GetFeatureKVStoreRecord(
			ctx, sqlc.GetFeatureKVStoreRecordParams{
				Key:       key,
				Perm:      s.params.perm,
				SessionID: sessionID,
				RuleID:    ruleID,
				FeatureID: featureID,
			},
		)

	case sessionID.Valid:
		return s.queries.GetSessionKVStoreRecord(
			ctx, sqlc.GetSessionKVStoreRecordParams{
				Key:       key,
				Perm:      s.params.perm,
				SessionID: sessionID,
				RuleID:    ruleID,
			},
		)

	case featureID.Valid:
		return nil, fmt.Errorf("a global feature kv store is " +
			"not currently supported")
	default:
		return s.queries.GetGlobalKVStoreRecord(
			ctx, sqlc.GetGlobalKVStoreRecordParams{
				Key:    key,
				Perm:   s.params.perm,
				RuleID: ruleID,
			},
		)
	}
}

func (s *sqlKVStore) Set(ctx context.Context, key string, value []byte) error {
	ruleID, sessionID, featureID, err := s.genNamespaceFields(ctx, false)
	if err != nil {
		return err
	}

	// We first need to figure out if we are inserting a new record or
	// updating an existing one. So first do a GET with the same set of
	// params.
	oldValue, err := s.get(ctx, key)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	// No such entry. Add new record.
	if errors.Is(err, sql.ErrNoRows) {
		return s.queries.InsertKVStoreRecord(
			ctx, sqlc.InsertKVStoreRecordParams{
				Key:       key,
				Value:     value,
				Perm:      s.params.perm,
				RuleID:    ruleID,
				SessionID: sessionID,
				FeatureID: featureID,
			},
		)
	}

	// If an entry exists but the value has not changed, there is nothing
	// left to do.
	if bytes.Equal(oldValue, value) {
		return nil
	}

	// Otherwise, the key exists but the value needs to be updated.
	switch {
	case sessionID.Valid && featureID.Valid:
		return s.queries.UpdateFeatureKVStoreRecord(
			ctx, sqlc.UpdateFeatureKVStoreRecordParams{
				Key:       key,
				Value:     value,
				Perm:      s.params.perm,
				SessionID: sessionID,
				RuleID:    ruleID,
				FeatureID: featureID,
			},
		)

	case sessionID.Valid:
		return s.queries.UpdateSessionKVStoreRecord(
			ctx, sqlc.UpdateSessionKVStoreRecordParams{
				Key:       key,
				Value:     value,
				Perm:      s.params.perm,
				SessionID: sessionID,
				RuleID:    ruleID,
			},
		)

	case featureID.Valid:
		return fmt.Errorf("a global feature kv store is " +
			"not currently supported")
	default:
		return s.queries.UpdateGlobalKVStoreRecord(
			ctx, sqlc.UpdateGlobalKVStoreRecordParams{
				Key:    key,
				Value:  value,
				Perm:   s.params.perm,
				RuleID: ruleID,
			},
		)
	}
}

func (s *sqlKVStore) Del(ctx context.Context, key string) error {
	// Note: we pass in true here for "read-only" since because this is a
	// Delete, if the record does not exist, we don't need to create one.
	// But no need to error out if it doesn't exist.
	ruleID, sessionID, featureID, err := s.genNamespaceFields(ctx, true)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}

	switch {
	case sessionID.Valid && featureID.Valid:
		return s.queries.DeleteFeatureKVStoreRecord(
			ctx, sqlc.DeleteFeatureKVStoreRecordParams{
				Key:       key,
				Perm:      s.params.perm,
				SessionID: sessionID,
				RuleID:    ruleID,
				FeatureID: featureID,
			},
		)

	case sessionID.Valid:
		return s.queries.DeleteSessionKVStoreRecord(
			ctx, sqlc.DeleteSessionKVStoreRecordParams{
				Key:       key,
				Perm:      s.params.perm,
				SessionID: sessionID,
				RuleID:    ruleID,
			},
		)

	case featureID.Valid:
		return fmt.Errorf("a global feature kv store is " +
			"not currently supported")
	default:
		return s.queries.DeleteGlobalKVStoreRecord(
			ctx, sqlc.DeleteGlobalKVStoreRecordParams{
				Key:    key,
				Perm:   s.params.perm,
				RuleID: ruleID,
			},
		)
	}
}

var _ KVStore = (*sqlKVStore)(nil)

package firewalldb

import (
	"bytes"
	"context"
	"database/sql"
	"errors"

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

type SQLQueries interface {
	SQLSessionQueries

	DeleteKVStoreRecord(ctx context.Context, arg sqlc.DeleteKVStoreRecordParams) error
	GetKVStoreRecord(ctx context.Context, arg sqlc.GetKVStoreRecordParams) ([]byte, error)
	InsertKVStoreRecord(ctx context.Context, arg sqlc.InsertKVStoreRecordParams) error
	DeleteAllTemp(ctx context.Context) error
	UpdateKVStoreRecord(ctx context.Context, arg sqlc.UpdateKVStoreRecordParams) error
}

// BatchedSQLQueries is a version of the SQLQueries that's capable of batched
// database operations.
type BatchedSQLQueries interface {
	SQLQueries

	db.BatchedTx[SQLQueries]
}

// SQLDB represents a storage backend.
type SQLDB struct {
	// db is all the higher level queries that the SQLStore has access to
	// in order to implement all its CRUD logic.
	db BatchedSQLQueries

	// BaseDB represents the underlying database connection.
	*db.BaseDB
}

var _ RulesDB = (*SQLDB)(nil)

// NewSQLDB creates a new SQLStore instance given an open BatchedSQLQueries
// storage backend.
func NewSQLDB(sqlDB *db.BaseDB) *SQLDB {
	executor := db.NewTransactionExecutor(
		sqlDB, func(tx *sql.Tx) SQLQueries {
			return sqlDB.WithTx(tx)
		},
	)

	return &SQLDB{
		db:     executor,
		BaseDB: sqlDB,
	}
}

func (s *SQLDB) DeleteTempKVStores(ctx context.Context) error {
	var writeTxOpts db.QueriesTxOptions

	return s.db.ExecTx(ctx, &writeTxOpts, func(tx SQLQueries) error {
		return tx.DeleteAllTemp(ctx)
	})
}

func (s *SQLDB) GetKVStores(rule string, groupAlias session.ID,
	feature string) KVStores {

	return &kvStoreSQLDB{
		SQLDB:      s,
		groupAlias: groupAlias,
		rule:       rule,
		feature:    feature,
	}
}

type kvStoreSQLDB struct {
	*SQLDB
	groupAlias session.ID
	rule       string
	feature    string
}

var _ DBExecutor[KVStoreTx] = (*kvStoreSQLDB)(nil)

func (k *kvStoreSQLDB) Update(ctx context.Context, fn func(ctx context.Context,
	tx KVStoreTx) error) error {

	var txOpts db.QueriesTxOptions
	return k.db.ExecTx(ctx, &txOpts, func(queries SQLQueries) error {
		sqlTx := &sqlKVStoresSQLTx{
			db:      k,
			queries: queries,
		}

		return fn(ctx, sqlTx)
	})
}

func (k *kvStoreSQLDB) View(ctx context.Context, fn func(ctx context.Context,
	tx KVStoreTx) error) error {

	txOpts := db.NewQueryReadTx()
	return k.db.ExecTx(ctx, &txOpts, func(queries SQLQueries) error {
		sqlTx := &sqlKVStoresSQLTx{
			db:      k,
			queries: queries,
		}

		return fn(ctx, sqlTx)
	})
}

type sqlKVStoresSQLTx struct {
	db      *kvStoreSQLDB
	queries SQLQueries
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

func (s *sqlKVStore) optionalFields(ctx context.Context) (sql.NullInt64,
	sql.NullString, error) {

	var (
		sessionID   sql.NullInt64
		featureName sql.NullString
		err         error
	)
	s.params.featureName.WhenSome(func(s string) {
		featureName = sql.NullString{
			String: s,
			Valid:  true,
		}
	})

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

	return sessionID, featureName, err
}

func (s *sqlKVStore) Get(ctx context.Context, key string) ([]byte, error) {
	sessionID, featureName, err := s.optionalFields(ctx)
	if err != nil {
		return nil, err
	}

	value, err := s.queries.GetKVStoreRecord(
		ctx, sqlc.GetKVStoreRecordParams{
			Key:         key,
			Perm:        s.params.perm,
			RuleName:    s.params.ruleName,
			SessionID:   sessionID,
			FeatureName: featureName,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return value, nil
}

func (s *sqlKVStore) Set(ctx context.Context, key string, value []byte) error {
	sessionID, featureName, err := s.optionalFields(ctx)
	if err != nil {
		return err
	}

	// We first need to figure out if we are inserting a new record or
	// updating an existing one. So first do a GET with the same set of
	// params.
	oldValue, err := s.queries.GetKVStoreRecord(
		ctx, sqlc.GetKVStoreRecordParams{
			Key:         key,
			Perm:        s.params.perm,
			RuleName:    s.params.ruleName,
			SessionID:   sessionID,
			FeatureName: featureName,
		},
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	// No such entry. Add new record.
	if errors.Is(err, sql.ErrNoRows) {
		return s.queries.InsertKVStoreRecord(
			ctx, sqlc.InsertKVStoreRecordParams{
				Key:         key,
				Value:       value,
				Perm:        s.params.perm,
				RuleName:    s.params.ruleName,
				SessionID:   sessionID,
				FeatureName: featureName,
			},
		)
	}

	// If an entry exists but the value has not changed, there is nothing
	// left to do.
	if bytes.Equal(oldValue, value) {
		return nil
	}

	// Otherwise, the key exists but the value needs to be updated.
	return s.queries.UpdateKVStoreRecord(
		ctx, sqlc.UpdateKVStoreRecordParams{
			Key:         key,
			Value:       value,
			Perm:        s.params.perm,
			RuleName:    s.params.ruleName,
			SessionID:   sessionID,
			FeatureName: featureName,
		},
	)
}

func (s *sqlKVStore) Del(ctx context.Context, key string) error {
	sessionID, featureName, err := s.optionalFields(ctx)
	if err != nil {
		return err
	}

	return s.queries.DeleteKVStoreRecord(ctx, sqlc.DeleteKVStoreRecordParams{
		Key:         key,
		Perm:        s.params.perm,
		RuleName:    s.params.ruleName,
		SessionID:   sessionID,
		FeatureName: featureName,
	})
}

var _ KVStore = (*sqlKVStore)(nil)

package firewalldb

import (
	"bytes"
	"context"
	"database/sql"
	"errors"

	"github.com/lightninglabs/lightning-terminal/db"
	"github.com/lightninglabs/lightning-terminal/db/sqlc"
	"github.com/lightninglabs/lightning-terminal/session"
)

type SQLKVStoresQueries interface {
	SQLSessionQueries

	DeleteKVStoreRecord(ctx context.Context, arg sqlc.DeleteKVStoreRecordParams) error
	GetKVStoreRecord(ctx context.Context, arg sqlc.GetKVStoreRecordParams) ([]byte, error)
	InsertKVStoreRecord(ctx context.Context, arg sqlc.InsertKVStoreRecordParams) error
	DeleteAllTemp(ctx context.Context) error
	UpdateKVStoreRecord(ctx context.Context, arg sqlc.UpdateKVStoreRecordParams) error
}

type SQLKVStoresDB struct {
	*db.BaseDB
}

var _ RulesDB = (*SQLKVStoresDB)(nil)

func NewSQLKVStoresDB(ctx context.Context, db *db.BaseDB) (*SQLKVStoresDB,
	error) {

	err := db.DeleteAllTemp(ctx)
	if err != nil {
		return nil, err
	}

	return &SQLKVStoresDB{
		BaseDB: db,
	}, nil
}

func (s *SQLKVStoresDB) GetKVStores(ctx context.Context, rule string,
	legacyGroupID session.ID, feature string) (KVStores, error) {

	groupID, err := s.GetSessionIDByLegacyID(ctx, legacyGroupID[:])
	if err != nil {
		return nil, err
	}

	return &dbExecutor[KVStoreTx]{
		db: &kvStoreSQLDB{
			BaseDB:  s.BaseDB,
			groupID: groupID,
			rule:    rule,
			feature: feature,
		},
	}, nil
}

type kvStoreSQLDB struct {
	*db.BaseDB
	groupID int64
	rule    string
	feature string
}

var _ txCreator[KVStoreTx] = (*kvStoreSQLDB)(nil)

func (k *kvStoreSQLDB) beginTx(ctx context.Context, writable bool) (KVStoreTx,
	error) {

	var txOpts SQLQueriesTxOptions
	if !writable {
		txOpts = NewSQLQueryReadTx()
	}

	sqlTX, err := k.BeginTx(ctx, &txOpts)
	if err != nil {
		return nil, err
	}

	return &sqlKVStoresSQLTx{
		db: k,
		Tx: sqlTX,
	}, nil
}

type sqlKVStoresSQLTx struct {
	db *kvStoreSQLDB
	*sql.Tx
}

func (s *sqlKVStoresSQLTx) IsNil() bool {
	return s.Tx == nil
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
	return &sqlKVStore{
		sqlKVStoresSQLTx: s,
		params: &sqlKVStoreParams{
			perm:     true,
			ruleName: s.db.rule,
			sessionID: sql.NullInt64{
				Int64: s.db.groupID,
				Valid: true,
			},
			featureName: sql.NullString{
				String: s.db.feature,
				Valid:  s.db.feature != "",
			},
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
	return &sqlKVStore{
		sqlKVStoresSQLTx: s,
		params: &sqlKVStoreParams{
			perm:     false,
			ruleName: s.db.rule,
			sessionID: sql.NullInt64{
				Int64: s.db.groupID,
				Valid: true,
			},
			featureName: sql.NullString{
				String: s.db.feature,
				Valid:  s.db.feature != "",
			},
		},
	}
}

var _ KVStoreTx = (*sqlKVStoresSQLTx)(nil)

type sqlKVStoreParams struct {
	perm        bool
	ruleName    string
	sessionID   sql.NullInt64
	featureName sql.NullString
}

func (s *sqlKVStore) Get(ctx context.Context, key string) ([]byte, error) {
	queries := SQLKVStoresQueries(s.db.WithTx(s.Tx))

	value, err := queries.GetKVStoreRecord(
		ctx, sqlc.GetKVStoreRecordParams{
			Key:         key,
			Perm:        s.params.perm,
			RuleName:    s.params.ruleName,
			SessionID:   s.params.sessionID,
			FeatureName: s.params.featureName,
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
	queries := SQLKVStoresQueries(s.db.WithTx(s.Tx))

	// We first need to figure out if we are inserting a new record or
	// updating an existing one. So first do a GET with the same set of
	// params.
	oldValue, err := queries.GetKVStoreRecord(
		ctx, sqlc.GetKVStoreRecordParams{
			Key:         key,
			Perm:        s.params.perm,
			RuleName:    s.params.ruleName,
			SessionID:   s.params.sessionID,
			FeatureName: s.params.featureName,
		},
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	// No such entry. Add new record.
	if errors.Is(err, sql.ErrNoRows) {
		return queries.InsertKVStoreRecord(
			ctx, sqlc.InsertKVStoreRecordParams{
				Key:         key,
				Value:       value,
				Perm:        s.params.perm,
				RuleName:    s.params.ruleName,
				SessionID:   s.params.sessionID,
				FeatureName: s.params.featureName,
			},
		)
	}

	// If an entry exists but the value has not changed, there is nothing
	// left to do.
	if bytes.Equal(oldValue, value) {
		return nil
	}

	// Otherwise, the key exists but the value needs to be updated.
	return queries.UpdateKVStoreRecord(
		ctx, sqlc.UpdateKVStoreRecordParams{
			Key:         key,
			Value:       value,
			Perm:        s.params.perm,
			RuleName:    s.params.ruleName,
			SessionID:   s.params.sessionID,
			FeatureName: s.params.featureName,
		},
	)
}

func (s *sqlKVStore) Del(ctx context.Context, key string) error {
	queries := SQLKVStoresQueries(s.db.WithTx(s.Tx))

	return queries.DeleteKVStoreRecord(ctx, sqlc.DeleteKVStoreRecordParams{
		Key:         key,
		Perm:        s.params.perm,
		RuleName:    s.params.ruleName,
		SessionID:   s.params.sessionID,
		FeatureName: s.params.featureName,
	})
}

var _ KVStore = (*sqlKVStore)(nil)

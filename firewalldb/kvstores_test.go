package firewalldb

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/lightninglabs/lightning-terminal/accounts"
	"github.com/lightninglabs/lightning-terminal/db"
	"github.com/lightninglabs/lightning-terminal/session"
	"github.com/stretchr/testify/require"
)

type testKVStoresDB struct {
	RulesDB
	session.Store

	recreateRulesDB func() RulesDB
}

func (db *testKVStoresDB) reopen() {
	db.RulesDB = db.recreateRulesDB()
}

func TestKVStoresDB(t *testing.T) {
	time.Local = time.UTC

	testList := []struct {
		name string
		test func(t *testing.T,
			makeDB func(t *testing.T) testKVStoresDB)
	}{
		{
			name: "KVStoreTxs",
			test: testKVStoreTxs,
		},
		{
			name: "TempAndPermStores",
			test: tempAndPermStores,
		},
		{
			name: "KVStoreNameSpaces",
			test: testKVStoreNameSpaces,
		},
	}

	makeKeyValueDBs := func(t *testing.T) testKVStoresDB {
		tmpDir := t.TempDir()

		sessionsDB, err := session.NewDB(tmpDir, "test_sessions.db")
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, sessionsDB.Close())
		})

		accountsDB, err := accounts.NewBoltStore(
			tmpDir, "test_accounts.db",
		)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, accountsDB.Close())
		})

		kvStoresDB, err := NewDB(
			tmpDir, "test_kvstores.db", sessionsDB, accountsDB,
		)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, kvStoresDB.Close())
		})

		return testKVStoresDB{
			RulesDB: kvStoresDB,
			Store:   sessionsDB,
			recreateRulesDB: func() RulesDB {
				require.NoError(t, kvStoresDB.Close())

				kvStoresDB, err := NewDB(
					tmpDir, "test_kvstores.db", sessionsDB,
					accountsDB,
				)
				require.NoError(t, err)
				t.Cleanup(func() {
					require.NoError(t, kvStoresDB.Close())
				})

				return kvStoresDB
			},
		}
	}

	// First create a shared Postgres instance so we don't spawn a new
	// docker container for each test.
	pgFixture := db.NewTestPgFixture(
		t, db.DefaultPostgresFixtureLifetime,
	)
	t.Cleanup(func() {
		pgFixture.TearDown(t)
	})

	makeSQLDB := func(t *testing.T, sqlite bool) testKVStoresDB {
		var sqlDB *db.BaseDB
		if sqlite {
			sqlDB = db.NewTestSqliteDB(t).BaseDB
		} else {
			sqlDB = db.NewTestPostgresDB(t, pgFixture).BaseDB
		}

		sessionsExecutor := db.NewTransactionExecutor(
			sqlDB, func(tx *sql.Tx) session.SQLQueries {
				return sqlDB.WithTx(tx)
			},
		)

		db, err := NewSQLKVStoresDB(context.TODO(), sqlDB)
		require.NoError(t, err)

		return testKVStoresDB{
			RulesDB: db,
			Store: session.NewSQLStore(
				sessionsExecutor,
			),
			recreateRulesDB: func() RulesDB {
				db, err := NewSQLKVStoresDB(context.TODO(), sqlDB)
				require.NoError(t, err)

				return db
			},
		}
	}

	for _, test := range testList {
		test := test
		t.Run(test.name+"_KV", func(t *testing.T) {
			test.test(t, makeKeyValueDBs)
		})

		// TODO(elle): fix sqlite time stamp issue.
		//t.Run(test.name+"_SQLite", func(t *testing.T) {
		//	test.test(t, func(t *testing.T) Store {
		//		return makeSQLDB(t, true)
		//	})
		//})

		t.Run(test.name+"_Postgres", func(t *testing.T) {
			test.test(t, func(t *testing.T) testKVStoresDB {
				return makeSQLDB(t, false)
			})
		})
	}
}

// testKVStoreTxs tests that the `Update` and `View` functions correctly provide
// atomic access to the db. If anything fails in the middle of an `Update`
// function, then all the changes prior should be rolled back.
func testKVStoreTxs(t *testing.T, makeDB func(t *testing.T) testKVStoresDB) {

	t.Parallel()
	ctx := context.Background()

	db := makeDB(t)

	session1 := newSession(t, db, nil)
	require.NoError(t, db.CreateSession(ctx, session1))

	store, err := db.GetKVStores(
		ctx, "AutoFees", session1.GroupID, "auto-fees",
	)
	require.NoError(t, err)

	// Test that if an action fails midway through the transaction, then
	// it is rolled back.
	err = store.Update(ctx, func(tx KVStoreTx) error {
		err := tx.Global().Set(ctx, "test", []byte{1})
		if err != nil {
			return err
		}

		b, err := tx.Global().Get(ctx, "test")
		if err != nil {
			return err
		}
		require.True(t, bytes.Equal(b, []byte{1}))

		// Now return an error.
		return fmt.Errorf("random error")
	})
	require.Error(t, err)

	var v []byte
	err = store.View(ctx, func(tx KVStoreTx) error {
		b, err := tx.Global().Get(ctx, "test")
		if err != nil {
			return err
		}
		v = b
		return nil
	})
	require.NoError(t, err)
	require.Nil(t, v)
}

// tempAndPermStores tests that the kv stores stored under the `temp` bucket
// are properly deleted and re-initialised upon restart but that anything under
// the `perm` bucket is retained. We repeat the test for both the session level
// KV stores and the session feature level stores.
func tempAndPermStores(t *testing.T,
	makeDB func(t *testing.T) testKVStoresDB) {

	t.Run("session level kv store", func(t *testing.T) {
		testTempAndPermStores(t, false, makeDB)
	})

	t.Run("session feature level kv store", func(t *testing.T) {
		testTempAndPermStores(t, true, makeDB)
	})
}

// testTempAndPermStores tests that the kv stores stored under the `temp` bucket
// are properly deleted and re-initialised upon restart but that anything under
// the `perm` bucket is retained. If featureSpecificStore is true, then this
// will test the session feature level KV stores. Otherwise, it will test the
// session level KV stores.
func testTempAndPermStores(t *testing.T, featureSpecificStore bool,
	makeDB func(t *testing.T) testKVStoresDB) {

	t.Parallel()
	ctx := context.Background()

	db := makeDB(t)

	var featureName string
	if featureSpecificStore {
		featureName = "auto-fees"
	}

	session1 := newSession(t, db, nil)
	require.NoError(t, db.CreateSession(ctx, session1))

	store, err := db.GetKVStores(
		ctx, "test-rule", session1.GroupID, featureName,
	)
	require.NoError(t, err)

	err = store.Update(ctx, func(tx KVStoreTx) error {
		// Set an item in the temp store.
		err := tx.LocalTemp().Set(ctx, "test", []byte{4, 3, 2})
		if err != nil {
			return err
		}

		// Set an item in the perm store.
		return tx.Local().Set(ctx, "test", []byte{6, 5, 4})
	})
	require.NoError(t, err)

	// Make sure that the newly added items are properly reflected _before_
	// restart.
	var (
		v1 []byte
		v2 []byte
	)
	err = store.View(ctx, func(tx KVStoreTx) error {
		b, err := tx.LocalTemp().Get(ctx, "test")
		if err != nil {
			return err
		}
		v1 = b

		b, err = tx.Local().Get(ctx, "test")
		if err != nil {
			return err
		}
		v2 = b
		return nil
	})
	require.NoError(t, err)
	require.True(t, bytes.Equal(v1, []byte{4, 3, 2}))
	require.True(t, bytes.Equal(v2, []byte{6, 5, 4}))

	// Restart the DB.
	db.reopen()

	store, err = db.GetKVStores(
		ctx, "test-rule", session1.GroupID, featureName,
	)
	require.NoError(t, err)

	// The temp store should no longer have the stored value but the perm
	// store should .
	err = store.View(ctx, func(tx KVStoreTx) error {
		b, err := tx.LocalTemp().Get(ctx, "test")
		if err != nil {
			return err
		}
		v1 = b

		b, err = tx.Local().Get(ctx, "test")
		if err != nil {
			return err
		}
		v2 = b
		return nil
	})
	require.NoError(t, err)
	require.Nil(t, v1)
	require.Equal(t, []byte{6, 5, 4}, v2)
}

// testKVStoreNameSpaces tests that the various name spaces are used correctly.
func testKVStoreNameSpaces(t *testing.T,
	makeDB func(t *testing.T) testKVStoresDB) {

	ctx := context.Background()

	db := makeDB(t)

	session1 := newSession(t, db, nil)
	require.NoError(t, db.CreateSession(ctx, session1))

	session2 := newSession(t, db, nil)
	require.NoError(t, db.CreateSession(ctx, session2))

	var (
		groupID1 = session1.GroupID
		groupID2 = session2.GroupID
	)

	// Two DBs for same group but different features.
	rulesDB1, err := db.GetKVStores(ctx, "test-rule", groupID1, "auto-fees")
	require.NoError(t, err)

	rulesDB2, err := db.GetKVStores(
		ctx, "test-rule", groupID1, "re-balance",
	)
	require.NoError(t, err)

	// The third DB is for the same rule but a different group. It is
	// for the same feature as db 2.
	rulesDB3, err := db.GetKVStores(
		ctx, "test-rule", groupID2, "re-balance",
	)
	require.NoError(t, err)

	// Test that the three ruleDBs share the same global space.
	err = rulesDB1.Update(ctx, func(tx KVStoreTx) error {
		return tx.Global().Set(
			ctx, "test-global", []byte("global thing!"),
		)
	})
	require.NoError(t, err)

	err = rulesDB2.Update(ctx, func(tx KVStoreTx) error {
		return tx.Global().Set(
			ctx, "test-global", []byte("different global thing!"),
		)
	})
	require.NoError(t, err)

	err = rulesDB3.Update(ctx, func(tx KVStoreTx) error {
		return tx.Global().Set(
			ctx, "test-global", []byte("yet another global thing"),
		)
	})
	require.NoError(t, err)

	err = rulesDB1.View(ctx, func(tx KVStoreTx) error {
		b, err := tx.Global().Get(ctx, "test-global")
		if err != nil {
			return err
		}
		require.Equal(t, []byte("yet another global thing"), b)
		return nil
	})
	require.NoError(t, err)

	err = rulesDB2.View(ctx, func(tx KVStoreTx) error {
		b, err := tx.Global().Get(ctx, "test-global")
		if err != nil {
			return err
		}
		require.Equal(t, []byte("yet another global thing"), b)
		return nil
	})
	require.NoError(t, err)

	err = rulesDB3.View(ctx, func(tx KVStoreTx) error {
		b, err := tx.Global().Get(ctx, "test-global")
		if err != nil {
			return err
		}
		require.Equal(t, []byte("yet another global thing"), b)
		return nil
	})
	require.NoError(t, err)

	// Test that the feature space is not shared by any of the dbs.
	err = rulesDB1.Update(ctx, func(tx KVStoreTx) error {
		return tx.Local().Set(ctx, "count", []byte("1"))
	})
	require.NoError(t, err)

	err = rulesDB2.Update(ctx, func(tx KVStoreTx) error {
		return tx.Local().Set(ctx, "count", []byte("2"))
	})
	require.NoError(t, err)

	err = rulesDB3.Update(ctx, func(tx KVStoreTx) error {
		return tx.Local().Set(ctx, "count", []byte("3"))
	})
	require.NoError(t, err)

	err = rulesDB1.View(ctx, func(tx KVStoreTx) error {
		b, err := tx.Local().Get(ctx, "count")
		if err != nil {
			return err
		}
		require.Equal(t, []byte("1"), b)
		return nil
	})
	require.NoError(t, err)

	err = rulesDB2.View(ctx, func(tx KVStoreTx) error {
		b, err := tx.Local().Get(ctx, "count")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		require.Equal(t, []byte("2"), b)
		return nil
	})
	require.NoError(t, err)

	err = rulesDB3.View(ctx, func(tx KVStoreTx) error {
		b, err := tx.Local().Get(ctx, "count")
		if err != nil {
			return err
		}
		require.Equal(t, []byte("3"), b)
		return nil
	})
	require.NoError(t, err)

	// Test that the group space is shared by the first two dbs but not
	// the third. To do this, we re-init the DB's but leave the feature
	// names out. This way, we will access the group storage.
	rulesDB1, err = db.GetKVStores(ctx, "test-rule", groupID1, "")
	require.NoError(t, err)
	rulesDB2, err = db.GetKVStores(ctx, "test-rule", groupID1, "")
	require.NoError(t, err)
	rulesDB3, err = db.GetKVStores(ctx, "test-rule", groupID2, "")
	require.NoError(t, err)

	err = rulesDB1.Update(ctx, func(tx KVStoreTx) error {
		return tx.Local().Set(ctx, "test", []byte("thing 1"))
	})
	require.NoError(t, err)

	err = rulesDB2.Update(ctx, func(tx KVStoreTx) error {
		return tx.Local().Set(ctx, "test", []byte("thing 2"))
	})
	require.NoError(t, err)

	err = rulesDB3.Update(ctx, func(tx KVStoreTx) error {
		return tx.Local().Set(ctx, "test", []byte("thing 3"))
	})
	require.NoError(t, err)

	var v []byte
	err = rulesDB1.View(ctx, func(tx KVStoreTx) error {
		b, err := tx.Local().Get(ctx, "test")
		if err != nil {
			return err
		}
		v = b
		return nil
	})
	require.NoError(t, err)
	require.True(t, bytes.Equal(v, []byte("thing 2")))

	err = rulesDB2.View(ctx, func(tx KVStoreTx) error {
		b, err := tx.Local().Get(ctx, "test")
		if err != nil {
			return err
		}
		v = b
		return nil
	})
	require.NoError(t, err)
	require.True(t, bytes.Equal(v, []byte("thing 2")))

	err = rulesDB3.View(ctx, func(tx KVStoreTx) error {
		b, err := tx.Local().Get(ctx, "test")
		if err != nil {
			return err
		}
		v = b
		return nil
	})
	require.NoError(t, err)
	require.True(t, bytes.Equal(v, []byte("thing 3")))
}

func intToSessionID(i uint32) session.ID {
	var id session.ID
	byteOrder.PutUint32(id[:], i)

	return id
}

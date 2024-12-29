package firewalldb

import (
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

type testPrivPairDB struct {
	PrivMapDBCreator
	session.Store
}

func TestPrivacyPairsDB(t *testing.T) {
	time.Local = time.UTC

	testList := []struct {
		name string
		test func(t *testing.T,
			makeDB func(t *testing.T) testPrivPairDB)
	}{
		{
			name: "PrivacyMapStorage",
			test: testPrivacyMapStorage,
		},
		{
			name: "PrivacyMapTxs",
			test: testPrivacyMapTxs,
		},
	}

	makeKeyValueDBs := func(t *testing.T) testPrivPairDB {
		tempDir := t.TempDir()

		sessionsDB, err := session.NewDB(tempDir, "test_sessions.db")
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, sessionsDB.Close())
		})

		accountsDB, err := accounts.NewBoltStore(
			tempDir, "test_accounts.db",
		)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, accountsDB.Close())
		})

		privPairDB, err := NewDB(
			tempDir, "test_priv_pairs.db", sessionsDB,
			accountsDB,
		)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, privPairDB.Close())
		})

		return testPrivPairDB{
			PrivMapDBCreator: privPairDB,
			Store:            sessionsDB,
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

	makeSQLDB := func(t *testing.T, sqlite bool) testPrivPairDB {
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

		return testPrivPairDB{
			PrivMapDBCreator: NewSQLPrivacyPairDB(sqlDB),
			Store:            session.NewSQLStore(sessionsExecutor),
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
			test.test(t, func(t *testing.T) testPrivPairDB {
				return makeSQLDB(t, false)
			})
		})
	}
}

// testPrivacyMapStorage tests the privacy mapper CRUD logic.
func testPrivacyMapStorage(t *testing.T,
	makeDB func(t *testing.T) testPrivPairDB) {

	t.Parallel()
	ctx := context.Background()

	db := makeDB(t)

	session1 := newSession(t, db, nil)

	var err error
	pdb1 := db.PrivacyDB(session1.GroupID)

	// We have not yet persisted the session, so we should get an "unknown
	// group" error if we try to query a privacy map pair for this group.
	_ = pdb1.Update(ctx, func(tx PrivacyMapTx) error {
		_, err = tx.RealToPseudo("real")
		require.ErrorIs(t, err, session.ErrUnknownGroup)

		return nil
	})

	// Persist the session.
	require.NoError(t, db.CreateSession(ctx, session1))

	_ = pdb1.Update(ctx, func(tx PrivacyMapTx) error {
		_, err = tx.RealToPseudo("real")
		require.ErrorIs(t, err, ErrNoSuchKeyFound)

		_, err = tx.PseudoToReal("pseudo")
		require.ErrorIs(t, err, ErrNoSuchKeyFound)

		err = tx.NewPair("real", "pseudo")
		require.NoError(t, err)

		pseudo, err := tx.RealToPseudo("real")
		require.NoError(t, err)
		require.Equal(t, "pseudo", pseudo)

		real, err := tx.PseudoToReal("pseudo")
		require.NoError(t, err)
		require.Equal(t, "real", real)

		pairs, err := tx.FetchAllPairs()
		require.NoError(t, err)

		require.EqualValues(t, pairs.pairs, map[string]string{
			"real": "pseudo",
		})

		return nil
	})

	session2 := newSession(t, db, nil)
	require.NoError(t, db.CreateSession(ctx, session2))
	pdb2 := db.PrivacyDB(session2.ID)

	_ = pdb2.Update(ctx, func(tx PrivacyMapTx) error {
		_, err = tx.RealToPseudo("real")
		require.ErrorIs(t, err, ErrNoSuchKeyFound)

		_, err = tx.PseudoToReal("pseudo")
		require.ErrorIs(t, err, ErrNoSuchKeyFound)

		err = tx.NewPair("real 2", "pseudo 2")
		require.NoError(t, err)

		pseudo, err := tx.RealToPseudo("real 2")
		require.NoError(t, err)
		require.Equal(t, "pseudo 2", pseudo)

		real, err := tx.PseudoToReal("pseudo 2")
		require.NoError(t, err)
		require.Equal(t, "real 2", real)

		pairs, err := tx.FetchAllPairs()
		require.NoError(t, err)

		require.EqualValues(t, pairs.pairs, map[string]string{
			"real 2": "pseudo 2",
		})

		return nil
	})

	session3 := newSession(t, db, nil)
	require.NoError(t, db.CreateSession(ctx, session3))
	pdb3 := db.PrivacyDB(session3.ID)

	_ = pdb3.Update(ctx, func(tx PrivacyMapTx) error {
		// Check that calling FetchAllPairs returns an empty map if
		// nothing exists in the DB yet.
		m, err := tx.FetchAllPairs()
		require.NoError(t, err)
		require.Empty(t, m.pairs)

		// Add a new pair.
		err = tx.NewPair("real 1", "pseudo 1")
		require.NoError(t, err)

		// Try to add a new pair that has the same real value as the
		// first pair. This should fail.
		err = tx.NewPair("real 1", "pseudo 2")
		require.ErrorIs(t, err, ErrDuplicateRealValue)

		// Try to add a new pair that has the same pseudo value as the
		// first pair. This should fail.
		err = tx.NewPair("real 2", "pseudo 1")
		require.ErrorIs(t, err, ErrDuplicatePseudoValue)

		// Add a few more pairs.
		err = tx.NewPair("real 2", "pseudo 2")
		require.NoError(t, err)

		err = tx.NewPair("real 3", "pseudo 3")
		require.NoError(t, err)

		err = tx.NewPair("real 4", "pseudo 4")
		require.NoError(t, err)

		// Check that FetchAllPairs correctly returns all the pairs.
		pairs, err := tx.FetchAllPairs()
		require.NoError(t, err)

		require.EqualValues(t, pairs.pairs, map[string]string{
			"real 1": "pseudo 1",
			"real 2": "pseudo 2",
			"real 3": "pseudo 3",
			"real 4": "pseudo 4",
		})

		// Do a few tests to ensure that the PrivacyMapPairs struct
		// returned from FetchAllPairs also works as expected.
		pseudo, ok := pairs.GetPseudo("real 1")
		require.True(t, ok)
		require.Equal(t, "pseudo 1", pseudo)

		// Fetch a real value that is not present.
		_, ok = pairs.GetPseudo("real 5")
		require.False(t, ok)

		// Try to add a conflicting pair.
		err = pairs.Add(map[string]string{"real 2": "pseudo 10"})
		require.ErrorContains(t, err, "cannot replace existing "+
			"pseudo entry for real value")

		// Add a new pair.
		err = pairs.Add(map[string]string{"real 5": "pseudo 5"})
		require.NoError(t, err)

		pseudo, ok = pairs.GetPseudo("real 5")
		require.True(t, ok)
		require.Equal(t, "pseudo 5", pseudo)

		// Finally, also test adding multiple new pairs with some
		// overlapping with previously added pairs.
		err = pairs.Add(map[string]string{
			// Add some pairs that already exist.
			"real 1": "pseudo 1",
			"real 3": "pseudo 3",
			// Add some new pairs.
			"real 6": "pseudo 6",
			"real 7": "pseudo 7",
		})
		require.NoError(t, err)

		// Verify that all the expected pairs can be found.
		for r, p := range map[string]string{
			"real 1": "pseudo 1",
			"real 2": "pseudo 2",
			"real 3": "pseudo 3",
			"real 4": "pseudo 4",
			"real 5": "pseudo 5",
			"real 6": "pseudo 6",
			"real 7": "pseudo 7",
		} {
			pseudo, ok = pairs.GetPseudo(r)
			require.True(t, ok)
			require.Equal(t, p, pseudo)
		}

		return nil
	})
}

// testPrivacyMapTxs tests that the `Update` and `View` functions correctly
// provide atomic access to the db. If anything fails in the middle of an
// `Update` function, then all the changes prior should be rolled back.
func testPrivacyMapTxs(t *testing.T, makeDB func(t *testing.T) testPrivPairDB) {
	t.Parallel()
	ctx := context.Background()

	db := makeDB(t)

	session1 := newSession(t, db, nil)
	require.NoError(t, db.CreateSession(ctx, session1))
	pdb1 := db.PrivacyDB(session1.ID)

	// Test that if an action fails midway through the transaction, then
	// it is rolled back.
	err := pdb1.Update(ctx, func(tx PrivacyMapTx) error {
		err := tx.NewPair("real", "pseudo")
		if err != nil {
			return err
		}

		p, err := tx.RealToPseudo("real")
		if err != nil {
			return err
		}
		require.Equal(t, "pseudo", p)

		// Now return an error.
		return fmt.Errorf("random error")
	})
	require.Error(t, err)

	err = pdb1.View(ctx, func(tx PrivacyMapTx) error {
		_, err := tx.RealToPseudo("real")
		return err
	})
	require.ErrorIs(t, err, ErrNoSuchKeyFound)
}

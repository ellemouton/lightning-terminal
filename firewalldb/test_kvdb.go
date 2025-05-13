//go:build !test_db_postgres && !test_db_sqlite

package firewalldb

import (
	"testing"

	"github.com/lightningnetwork/lnd/clock"
	"github.com/stretchr/testify/require"
)

// NewTestDB is a helper function that creates an BBolt database for testing.
func NewTestDB(t *testing.T, clock clock.Clock) *BoltDB {
	return NewTestDBFromPath(t, t.TempDir(), clock)
}

// NewTestDBFromPath is a helper function that creates a new BoltStore with a
// connection to an existing BBolt database for testing.
func NewTestDBFromPath(t *testing.T, dbPath string, clock clock.Clock) *BoltDB {
	return newDBFromPathWithSessions(t, dbPath, nil, nil, clock)
}

// NewTestDBWithSessions creates a new test BoltDB Store with access to an
// existing sessions DB.
func NewTestDBWithSessions(t *testing.T, sessStore SessionDB,
	clock clock.Clock) *BoltDB {

	return newDBFromPathWithSessions(t, t.TempDir(), sessStore, nil, clock)
}

func newDBFromPathWithSessions(t *testing.T, dbPath string,
	sessStore SessionDB, acctStore AccountsDB, clock clock.Clock) *BoltDB {

	store, err := NewBoltDB(dbPath, DBFilename, sessStore, acctStore, clock)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, store.DB.Close())
	})

	return store
}

func assertEqualActions(t *testing.T, expected, got *Action) {
	// Accounts are not explicitly linked in our bbolt DB implementation.
	got.AccountID = expected.AccountID

	require.Equal(t, expected, got)
}

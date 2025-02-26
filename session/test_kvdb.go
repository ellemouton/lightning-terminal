//go:build !test_db_postgres && !test_db_sqlite

package session

import (
	"errors"
	"testing"

	"github.com/lightninglabs/lightning-terminal/accounts"
	"github.com/lightningnetwork/lnd/clock"
	"github.com/stretchr/testify/require"
)

// ErrDBClosed is an error that is returned when a database operation is
// performed on a closed database.
var ErrDBClosed = errors.New("database not open")

// NewTestDB is a helper function that creates an BBolt database for testing.
func NewTestDB(t *testing.T, clock clock.Clock) *BoltStore {
	return NewTestDBFromPath(t, t.TempDir(), clock)
}

// NewTestDBFromPath is a helper function that creates a new BoltStore with a
// connection to an existing BBolt database for testing.
func NewTestDBFromPath(t *testing.T, dbPath string,
	clock clock.Clock) *BoltStore {

	acctStore := accounts.NewTestDB(t, clock)

	store, err := NewDB(dbPath, DBFilename, clock, acctStore)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, store.DB.Close())
	})

	return store
}

func NewTestDBWithAccounts(t *testing.T, clock clock.Clock,
	acctStore accounts.Store) *BoltStore {

	store, err := NewDB(t.TempDir(), DBFilename, clock, acctStore)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, store.DB.Close())
	})

	return store
}

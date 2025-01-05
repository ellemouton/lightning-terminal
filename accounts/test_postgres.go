//go:build test_db_postgres && !test_db_sqlite

package accounts

import (
	"errors"
	"testing"

	"github.com/lightninglabs/lightning-terminal/db"
)

// ErrDBClosed is an error that is returned when a database operation is
// performed on a closed database.
var ErrDBClosed = errors.New("database is closed")

// NewTestDB is a helper function that creates an BBolt database for testing.
func NewTestDB(t *testing.T) *SQLStore {
	return NewSQLStore(db.NewTestPostgresDB(t).BaseDB)
}

// NewTestDBFromPath is a helper function that creates a new BoltStore with a
// connection to an existing BBolt database for testing.
func NewTestDBFromPath(t *testing.T, dbPath string) *SQLStore {
	return NewSQLStore(db.NewTestPostgresDB(t).BaseDB)
}

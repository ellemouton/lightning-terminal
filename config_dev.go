//go:build dev

package terminal

import (
	"path/filepath"

	"github.com/lightninglabs/lightning-terminal/accounts"
	"github.com/lightninglabs/lightning-terminal/db"
)

const (
	// DatabaseBackendSqlite is the name of the SQLite database backend.
	DatabaseBackendSqlite = "sqlite"

	// DatabaseBackendPostgres is the name of the Postgres database backend.
	DatabaseBackendPostgres = "postgres"

	// DatabaseBackendBbolt is the name of the bbolt database backend.
	DatabaseBackendBbolt = "bbolt"

	// defaultSqliteDatabaseFileName is the default name of the SQLite
	// database file.
	defaultSqliteDatabaseFileName = "litd.db"
)

// defaultSqliteDatabasePath is the default path under which we store
// the SQLite database file.
var defaultSqliteDatabasePath = filepath.Join(
	DefaultLitDir, DefaultNetwork, defaultSqliteDatabaseFileName,
)

// DevConfig is a struct that holds the configuration options for a development
// environment. The purpose of this struct is to hold config options for
// features not yet available in production. Since our itests are built with
// the dev tag, we can test these features in our itests.
//
// nolint:lll
type DevConfig struct {
	// DatabaseBackend is the database backend we will use for storing all
	// account related data. While this feature is still in development, we
	// include the bbolt type here so that our itests can continue to be
	// tested against a bbolt backend. Once the full bbolt to SQL migration
	// is complete, however, we will remove the bbolt option.
	DatabaseBackend string `long:"databasebackend" description:"The database backend to use for storing all account related data." choice:"bbolt" choice:"sqlite" choice:"postgres"`

	// Sqlite holds the configuration options for a SQLite database
	// backend.
	Sqlite *db.SqliteConfig `group:"sqlite" namespace:"sqlite"`

	// Postgres holds the configuration options for a Postgres database
	Postgres *db.PostgresConfig `group:"postgres" namespace:"postgres"`
}

// Validate checks that all the values set in our DevConfig are valid and uses
// the passed parameters to override any defaults if necessary.
func (c *DevConfig) Validate(dbDir, network string) error {
	// We'll update the database file location if it wasn't set.
	if c.Sqlite.DatabaseFileName == defaultSqliteDatabasePath {
		c.Sqlite.DatabaseFileName = filepath.Join(
			dbDir, network, defaultSqliteDatabaseFileName,
		)
	}

	return nil
}

// defaultDevConfig returns a new DevConfig with default values set.
func defaultDevConfig() *DevConfig {
	return &DevConfig{
		Sqlite: &db.SqliteConfig{
			DatabaseFileName: defaultSqliteDatabasePath,
		},
		Postgres: &db.PostgresConfig{
			Host:               "localhost",
			Port:               5432,
			MaxOpenConnections: 10,
		},
	}
}

// NewAccountStore creates a new account store based on the chosen database
// backend.
func NewAccountStore(cfg *Config) (accounts.Store, error) {
	switch cfg.DatabaseBackend {
	case DatabaseBackendSqlite:
		// Before we initialize the SQLite store, we'll make sure that
		// the directory where we will store the database file exists.
		networkDir := filepath.Join(cfg.LitDir, cfg.Network)
		err := makeDirectories(networkDir)
		if err != nil {
			return nil, err
		}

		sqlStore, err := db.NewSqliteStore(cfg.Sqlite)
		if err != nil {
			return nil, err
		}

		return accounts.NewSQLStore(sqlStore.BaseDB), nil

	case DatabaseBackendPostgres:
		sqlStore, err := db.NewPostgresStore(cfg.Postgres)
		if err != nil {
			return nil, err
		}

		return accounts.NewSQLStore(sqlStore.BaseDB), nil

	default:
		return accounts.NewBoltStore(
			filepath.Dir(cfg.MacaroonPath), accounts.DBFilename,
		)
	}
}

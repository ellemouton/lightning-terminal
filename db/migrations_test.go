package db

import (
	"testing"
)

// makeMigrationTestDB is a type alias for a function that creates a new test
// database and returns the base database and a function that executes selected
// migrations.
type makeMigrationTestDB = func(*testing.T, uint) (*BaseDB,
	func(MigrationTarget) error)

// TestMigrations is a meta test runner that runs all migration tests.
func TestMigrations(t *testing.T) {
	sqliteTestDB := func(t *testing.T, version uint) (*BaseDB,
		func(MigrationTarget) error) {

		db := NewTestSqliteDBWithVersion(t, version)

		return db.BaseDB, db.ExecuteMigrations
	}

	postgresTestDB := func(t *testing.T, version uint) (*BaseDB,
		func(MigrationTarget) error) {

		pgFixture := NewTestPgFixture(t, DefaultPostgresFixtureLifetime)
		t.Cleanup(func() {
			pgFixture.TearDown(t)
		})

		db := NewTestPostgresDBWithVersion(
			t, pgFixture, version,
		)

		return db.BaseDB, db.ExecuteMigrations
	}

	var tests []struct {
		name string
		test func(*testing.T, makeMigrationTestDB)
	}

	for _, test := range tests {
		test := test

		t.Run(test.name+"_SQLite", func(t *testing.T) {
			test.test(t, sqliteTestDB)
		})

		t.Run(test.name+"_Postgres", func(t *testing.T) {
			test.test(t, postgresTestDB)
		})
	}
}

package firewalldb

import (
	"context"
	"fmt"
	"sync"

	"github.com/lightningnetwork/lnd/fn"
)

var (
	// ErrNoSuchKeyFound is returned when there is no key-value pair found
	// for the given key.
	ErrNoSuchKeyFound = fmt.Errorf("no such key found")
)

type firewallDBs interface {
	RulesDB
	PrivacyMapper
}

// DB manages the firewall rules database.
type DB struct {
	started sync.Once
	stopped sync.Once

	firewallDBs

	cancel fn.Option[context.CancelFunc]
}

// NewDB creates a new firewall database. For now, it only contains the
// underlying rules' database.
func NewDB(dbs firewallDBs) *DB {
	return &DB{
		firewallDBs: dbs,
	}
}

// Start starts the firewall database.
func (db *DB) Start(ctx context.Context) error {
	db.started.Do(func() {
		_, cancel := context.WithCancel(ctx)
		db.cancel = fn.Some(cancel)
	})

	return db.DeleteTempKVStores(ctx)
}

// Stop stops the firewall database operations.
func (db *DB) Stop() error {
	db.stopped.Do(func() {})

	return nil
}

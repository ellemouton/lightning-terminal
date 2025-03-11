package firewalldb

import (
	"context"
	"sync/atomic"
)

// DB manages the firewall rules database.
type DB struct {
	started atomic.Bool

	RulesDB
}

// NewDB creates a new firewall database. For now, it only contains the
// underlying rules' database.
func NewDB(kvdb RulesDB) *DB {
	return &DB{
		RulesDB: kvdb,
	}
}

// Start starts the firewall database.
func (db *DB) Start(ctx context.Context) error {
	if !db.started.CompareAndSwap(false, true) {
		return nil
	}

	return db.DeleteTempKVStores(ctx)
}

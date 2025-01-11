package firewalldb

import (
	"context"

	"github.com/lightninglabs/lightning-terminal/session"
)

type KVStores = DBExecutor[KVStoreTx]

// KVStoreTx represents a database transaction that can be used for both read
// and writes of the various different key value stores offered for the rule.
type KVStoreTx interface {
	NullableTx

	// Global returns a persisted global, rule-name indexed, kv store. A
	// rule with a given name will have access to this store independent of
	// group ID or feature.
	Global() KVStore

	// Local returns a persisted local kv store for the rule. Depending on
	// how the implementation is initialised, this will either be under the
	// group ID namespace or the group ID _and_ feature name namespace.
	Local() KVStore

	// GlobalTemp is similar to the Global store except that its contents
	// is cleared upon restart of the database. The reason persisting the
	// temporary store changes instead of just keeping an in-memory store is
	// that we can then guarantee atomicity if changes are made to both
	// the permanent and temporary stores.
	GlobalTemp() KVStore

	// LocalTemp is similar to the Local store except that its contents is
	// cleared upon restart of the database. The reason persisting the
	// temporary store changes instead of just keeping an in-memory store is
	// that we can then guarantee atomicity if changes are made to both
	// the permanent and temporary stores.
	LocalTemp() KVStore
}

// KVStore is in interface representing a key value store. It allows us to
// abstract away the details of the data storage method.
type KVStore interface {
	// Get fetches the value under the given key from the underlying kv
	// store. If no value is found, nil is returned.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set sets the given key-value pair in the underlying kv store.
	Set(ctx context.Context, key string, value []byte) error

	// Del deletes the value under the given key in the underlying kv store.
	Del(ctx context.Context, key string) error
}

// RulesDB can be used to initialise a new rules.KVStores.
type RulesDB interface {
	GetKVStores(ctx context.Context, rule string, groupID session.ID,
		feature string) (KVStores, error)
}

// kvStores implements the rules.KVStores interface.
type kvStores struct {
	*DB
	ruleName    string
	groupID     session.ID
	featureName string
}

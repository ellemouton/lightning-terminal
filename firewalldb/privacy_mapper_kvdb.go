package firewalldb

import (
	"context"
	"fmt"

	"github.com/lightninglabs/lightning-terminal/session"
	"go.etcd.io/bbolt"
)

/*
	The PrivacyMapper data is stored in the following structure in the db:

	privacy -> group id -> real-to-pseudo -> {k:v}
			    -> pseudo-to-real -> {k:v}
*/

var (
	privacyBucketKey = []byte("privacy")
	realToPseudoKey  = []byte("real-to-pseudo")
	pseudoToRealKey  = []byte("pseudo-to-real")

	pseudoStrAlphabet    = []rune("abcdef0123456789")
	pseudoStrAlphabetLen = len(pseudoStrAlphabet)
)

// PrivacyDB constructs a PrivacyMapDB that will be indexed under the given
// group ID key.
func (db *DB) PrivacyDB(groupID session.ID) PrivacyMapDB {
	return &privacyMapDB{
		db:      db,
		groupID: groupID,
	}
}

// privacyMapDB is an implementation of PrivacyMapDB.
type privacyMapDB struct {
	db      *DB
	groupID session.ID
}

// beginTx starts db transaction. The transaction will be a read or read-write
// transaction depending on the value of the `writable` parameter.
func (p *privacyMapDB) beginTx(writable bool) (*privacyMapTx, error) {
	boltTx, err := p.db.Begin(writable)
	if err != nil {
		return nil, err
	}
	return &privacyMapTx{
		privacyMapDB: p,
		boltTx:       boltTx,
	}, nil
}

// Update opens a database read/write transaction and executes the function f
// with the transaction passed as a parameter. After f exits, if f did not
// error, the transaction is committed. Otherwise, if f did error, the
// transaction is rolled back. If the rollback fails, the original error
// returned by f is still returned. If the commit fails, the commit error is
// returned.
//
// NOTE: this is part of the PrivacyMapDB interface.
func (p *privacyMapDB) Update(_ context.Context,
	f func(tx PrivacyMapTx) error) error {

	tx, err := p.beginTx(true)
	if err != nil {
		return err
	}

	// Make sure the transaction rolls back in the event of a panic.
	defer func() {
		if tx != nil {
			_ = tx.boltTx.Rollback()
		}
	}()

	err = f(tx)
	if err != nil {
		// Want to return the original error, not a rollback error if
		// any occur.
		_ = tx.boltTx.Rollback()
		return err
	}

	return tx.boltTx.Commit()
}

// View opens a database read transaction and executes the function f with the
// transaction passed as a parameter. After f exits, the transaction is rolled
// back. If f errors, its error is returned, not a rollback error (if any
// occur).
//
// NOTE: this is part of the PrivacyMapDB interface.
func (p *privacyMapDB) View(_ context.Context,
	f func(tx PrivacyMapTx) error) error {

	tx, err := p.beginTx(false)
	if err != nil {
		return err
	}

	// Make sure the transaction rolls back in the event of a panic.
	defer func() {
		if tx != nil {
			_ = tx.boltTx.Rollback()
		}
	}()

	err = f(tx)
	rollbackErr := tx.boltTx.Rollback()
	if err != nil {
		return err
	}

	if rollbackErr != nil {
		return rollbackErr
	}
	return nil
}

// privacyMapTx is an implementation of PrivacyMapTx.
type privacyMapTx struct {
	*privacyMapDB
	boltTx *bbolt.Tx
}

// NewPair inserts a new real-pseudo pair into the db.
//
// NOTE: this is part of the PrivacyMapTx interface.
func (p *privacyMapTx) NewPair(real, pseudo string) error {
	privacyBucket, err := getBucket(p.boltTx, privacyBucketKey)
	if err != nil {
		return err
	}

	sessBucket, err := privacyBucket.CreateBucketIfNotExists(p.groupID[:])
	if err != nil {
		return err
	}

	realToPseudoBucket, err := sessBucket.CreateBucketIfNotExists(
		realToPseudoKey,
	)
	if err != nil {
		return err
	}

	pseudoToRealBucket, err := sessBucket.CreateBucketIfNotExists(
		pseudoToRealKey,
	)
	if err != nil {
		return err
	}

	if len(realToPseudoBucket.Get([]byte(real))) != 0 {
		return fmt.Errorf("an entry already exists for real "+
			"value: %x", real)
	}

	if len(pseudoToRealBucket.Get([]byte(pseudo))) != 0 {
		return fmt.Errorf("an entry already exists for pseudo "+
			"value: %x", pseudo)
	}

	err = realToPseudoBucket.Put([]byte(real), []byte(pseudo))
	if err != nil {
		return err
	}

	return pseudoToRealBucket.Put([]byte(pseudo), []byte(real))
}

// PseudoToReal will check the db to see if the given pseudo key exists. If
// it does then the real value is returned, else an error is returned.
//
// NOTE: this is part of the PrivacyMapTx interface.
func (p *privacyMapTx) PseudoToReal(pseudo string) (string, error) {
	privacyBucket, err := getBucket(p.boltTx, privacyBucketKey)
	if err != nil {
		return "", err
	}

	sessBucket := privacyBucket.Bucket(p.groupID[:])
	if sessBucket == nil {
		return "", ErrNoSuchKeyFound
	}

	pseudoToRealBucket := sessBucket.Bucket(pseudoToRealKey)
	if pseudoToRealBucket == nil {
		return "", ErrNoSuchKeyFound
	}

	real := pseudoToRealBucket.Get([]byte(pseudo))
	if len(real) == 0 {
		return "", ErrNoSuchKeyFound
	}

	return string(real), nil
}

// RealToPseudo will check the db to see if the given real key exists. If
// it does then the pseudo value is returned, else an error is returned.
//
// NOTE: this is part of the PrivacyMapTx interface.
func (p *privacyMapTx) RealToPseudo(real string) (string, error) {
	privacyBucket, err := getBucket(p.boltTx, privacyBucketKey)
	if err != nil {
		return "", err
	}

	sessBucket := privacyBucket.Bucket(p.groupID[:])
	if sessBucket == nil {
		return "", ErrNoSuchKeyFound
	}

	realToPseudoBucket := sessBucket.Bucket(realToPseudoKey)
	if realToPseudoBucket == nil {
		return "", ErrNoSuchKeyFound
	}

	pseudo := realToPseudoBucket.Get([]byte(real))
	if len(pseudo) == 0 {
		return "", ErrNoSuchKeyFound
	}

	return string(pseudo), nil
}

// FetchAllPairs loads and returns the real-to-pseudo pairs.
//
// NOTE: this is part of the PrivacyMapTx interface.
func (p *privacyMapTx) FetchAllPairs() (*PrivacyMapPairs, error) {
	privacyBucket, err := getBucket(p.boltTx, privacyBucketKey)
	if err != nil {
		return nil, err
	}

	sessBucket := privacyBucket.Bucket(p.groupID[:])
	if sessBucket == nil {
		// If the bucket has not been created yet, then there are no
		// privacy pairs yet.
		return NewPrivacyMapPairs(nil), nil
	}

	realToPseudoBucket := sessBucket.Bucket(realToPseudoKey)
	if realToPseudoBucket == nil {
		return nil, ErrNoSuchKeyFound
	}

	pairs := make(map[string]string)
	err = realToPseudoBucket.ForEach(func(r, p []byte) error {
		pairs[string(r)] = string(p)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return NewPrivacyMapPairs(pairs), nil
}

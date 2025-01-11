package firewalldb

import (
	"context"

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

// PrivacyDB constructs a DBExecutor that will be indexed under the given
// group ID key.
func (db *DB) PrivacyDB(groupID session.ID) PrivacyMapDB {
	return &dbExecutor[PrivacyMapTx]{
		db: &privacyMapKVDBDB{
			DB:             db,
			groupID:        groupID,
			sessionIDIndex: db.sessionIDIndex,
		},
	}
}

var _ PrivMapDBCreator = (*DB)(nil)

type privacyMapKVDBDB struct {
	*DB
	groupID        session.ID
	sessionIDIndex SessionDB
}

var _ txCreator[PrivacyMapTx] = (*privacyMapKVDBDB)(nil)

// beginTx starts db transaction. The transaction will be a read or read-write
// transaction depending on the value of the `writable` parameter.
func (p *privacyMapKVDBDB) beginTx(_ context.Context, writable bool) (
	PrivacyMapTx, error) {

	boltTx, err := p.Begin(writable)
	if err != nil {
		return nil, err
	}

	return &privacyMapTx{
		privacyMapKVDBDB: p,
		Tx:               boltTx,
	}, nil
}

// privacyMapTx is an implementation of PrivacyMapTx.
type privacyMapTx struct {
	*privacyMapKVDBDB
	*bbolt.Tx
}

func (p *privacyMapTx) IsNil() bool {
	return p.Tx == nil
}

// NewPair inserts a new real-pseudo pair into the db.
//
// NOTE: this is part of the PrivacyMapTx interface.
func (p *privacyMapTx) NewPair(real, pseudo string) error {
	privacyBucket, err := getBucket(p.Tx, privacyBucketKey)
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
		return ErrDuplicateRealValue
	}

	if len(pseudoToRealBucket.Get([]byte(pseudo))) != 0 {
		return ErrDuplicatePseudoValue
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
	privacyBucket, err := getBucket(p.Tx, privacyBucketKey)
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
	ctx := context.TODO()

	// First, check that this session actually exists in the sessions DB.
	_, err := p.sessionIDIndex.GetSessionIDs(ctx, p.groupID)
	if err != nil {
		return "", err
	}

	privacyBucket, err := getBucket(p.Tx, privacyBucketKey)
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
	privacyBucket, err := getBucket(p.Tx, privacyBucketKey)
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

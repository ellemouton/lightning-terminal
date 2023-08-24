package migration1

import (
	"bytes"
	"errors"
	"fmt"

	"go.etcd.io/bbolt"
)

var (
	// sessionBucketKey is the top level bucket where we can find all
	// information about sessions. These sessions are indexed by their
	// public key.
	//
	// The session bucket has the following structure:
	// session -> <key>       -> <serialised session>
	//	   -> id-index    -> <session-id> -> key -> <session key>
	sessionBucketKey = []byte("session")

	// idIndexKey is the key used to define the id-index sub-bucket within
	// the main session bucket. This bucket will be used to store the
	// mapping from session ID to various other fields.
	idIndexKey = []byte("id-index")

	// sessionKeyKey is the key used within the id-index bucket to store the
	// session key (serialised local public key) associated with the given
	// session ID.
	sessionKeyKey = []byte("key")

	// ErrDBInitErr is returned when a bucket that we expect to have been
	// set up during DB initialisation is not found.
	ErrDBInitErr = errors.New("db did not initialise properly")
)

// MigrateSessionIDToKeyIndex back-fills the session ID to key index so that it
// has an entry for all sessions that the session store is currently aware of.
func MigrateSessionIDToKeyIndex(tx *bbolt.Tx) error {
	sessionBucket := tx.Bucket(sessionBucketKey)
	if sessionBucket == nil {
		return fmt.Errorf("session bucket not found")
	}

	idIndexBkt := sessionBucket.Bucket(idIndexKey)
	if idIndexBkt == nil {
		return ErrDBInitErr
	}

	// Collect all the index entries.
	return sessionBucket.ForEach(func(key, sessionBytes []byte) error {
		// The session bucket contains both keys and sub-buckets. So
		// here we ensure that we skip any sub-buckets.
		if len(sessionBytes) == 0 {
			return nil
		}

		session, err := DeserializeSession(
			bytes.NewReader(sessionBytes),
		)
		if err != nil {
			return err
		}

		idBkt, err := idIndexBkt.CreateBucket(session.ID[:])
		if err != nil {
			return err
		}

		return idBkt.Put(sessionKeyKey[:], key)
	})
}

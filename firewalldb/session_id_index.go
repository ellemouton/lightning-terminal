package firewalldb

import (
	"fmt"

	"github.com/lightninglabs/lightning-terminal/session"
	"go.etcd.io/bbolt"
)

/*
	session-id-index -> <session id> -> <group id>
*/

var sessionIDIndexBucketKey = []byte("session-id-index")

type SessionIDIndex interface {
	AddGroupID(sessionID, groupID session.ID) error
	GetGroupID(sessionID session.ID) (session.ID, error)
}

func (db *DB) AddGroupID(sessionID, groupID session.ID) error {
	return db.DB.Update(func(tx *bbolt.Tx) error {
		indexBkt, err := getBucket(tx, sessionIDIndexBucketKey)
		if err != nil {
			return err
		}

		return indexBkt.Put(sessionID[:], groupID[:])
	})
}

func (db *DB) GetGroupID(sessionID session.ID) (session.ID, error) {
	var groupID session.ID
	err := db.DB.View(func(tx *bbolt.Tx) error {
		indexBkt, err := getBucket(tx, sessionIDIndexBucketKey)
		if err != nil {
			return err
		}

		groupIDBytes := indexBkt.Get(sessionID[:])
		if len(groupIDBytes) == 0 {
			return fmt.Errorf("group ID not found for session "+
				"ID %x", sessionID)
		}

		copy(groupID[:], groupIDBytes)

		return nil
	})
	if err != nil {
		return groupID, err
	}

	return groupID, nil
}

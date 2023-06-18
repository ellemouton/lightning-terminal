package firewalldb

import (
	"fmt"

	"github.com/lightninglabs/lightning-terminal/session"
	"go.etcd.io/bbolt"
)

/*
	session-id-index -> sess-to-group -> <session id> -> <group id>
			 -> group-to-sess -> <group id> -> id -> session id
*/

var (
	sessionIDIndexBucketKey = []byte("session-id-index")
	sessionToGroupKey       = []byte("session-to-group")
	groupToSessionKey       = []byte("group-to-session")
)

type SessionIDIndex interface {
	AddGroupID(sessionID, groupID session.ID) error
	GetGroupID(sessionID session.ID) (session.ID, error)
	GetSessionIDs(groupID session.ID) ([]session.ID, error)
}

func (db *DB) AddGroupID(sessionID, groupID session.ID) error {
	return db.DB.Update(func(tx *bbolt.Tx) error {
		indexBkt, err := getBucket(tx, sessionIDIndexBucketKey)
		if err != nil {
			return err
		}

		sessToGroupBkt := indexBkt.Bucket(sessionToGroupKey)
		if sessToGroupBkt == nil {
			return ErrDBInitErr
		}

		groupToSessionBkt := indexBkt.Bucket(groupToSessionKey)
		if groupToSessionBkt == nil {
			return ErrDBInitErr
		}

		groupBkt, err := groupToSessionBkt.CreateBucketIfNotExists(
			groupID[:],
		)
		if err != nil {
			return err
		}

		nextSeq, err := groupBkt.NextSequence()
		if err != nil {
			return err
		}

		var seqNoBytes [8]byte
		byteOrder.PutUint64(seqNoBytes[:], nextSeq)

		err = groupBkt.Put(seqNoBytes[:], sessionID[:])
		if err != nil {
			return err
		}

		return sessToGroupBkt.Put(sessionID[:], groupID[:])
	})
}

func (db *DB) GetGroupID(sessionID session.ID) (session.ID, error) {
	var groupID session.ID
	err := db.DB.View(func(tx *bbolt.Tx) error {
		indexBkt, err := getBucket(tx, sessionIDIndexBucketKey)
		if err != nil {
			return err
		}

		sessToGroupBkt := indexBkt.Bucket(sessionToGroupKey)
		if sessToGroupBkt == nil {
			return ErrDBInitErr
		}

		groupIDBytes := sessToGroupBkt.Get(sessionID[:])
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

func (db *DB) GetSessionIDs(groupID session.ID) ([]session.ID, error) {
	var sessionIDs []session.ID
	err := db.DB.View(func(tx *bbolt.Tx) error {
		indexBkt, err := getBucket(tx, sessionIDIndexBucketKey)
		if err != nil {
			return err
		}

		groupToSessionBkt := indexBkt.Bucket(groupToSessionKey)
		if groupToSessionBkt == nil {
			return ErrDBInitErr
		}

		groupBkt := groupToSessionBkt.Bucket(groupID[:])
		if groupBkt == nil {
			return nil
		}

		return groupBkt.ForEach(func(_, sessionIDBytes []byte) error {
			var sessionID session.ID
			copy(sessionID[:], sessionIDBytes)
			sessionIDs = append(sessionIDs, sessionID)

			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return sessionIDs, nil
}

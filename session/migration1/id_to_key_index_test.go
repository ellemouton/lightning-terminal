package migration1

import (
	"bytes"
	"testing"
	"time"

	"github.com/lightninglabs/lightning-terminal/session/migtest"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
)

// TestMigrateSessionIDIndex tests that the MigrateSessionIDIndex migration
// correctly back-fills the session-ID to session-key index.
func TestMigrateSessionIDIndex(t *testing.T) {
	t.Parallel()

	// Make a few sessions.
	sess1ID, sess1Key, sess1Bytes := newSession(t)
	sess2ID, sess2Key, sess2Bytes := newSession(t)
	sess3ID, sess3Key, sess3Bytes := newSession(t)

	// Put together a sample session DB based on the above.
	sessionDBBefore := map[string]interface{}{
		string(sess1Key):   string(sess1Bytes),
		string(sess2Key):   string(sess2Bytes),
		string(sess3Key):   string(sess3Bytes),
		string(idIndexKey): map[string]interface{}{},
	}

	before := func(tx *bbolt.Tx) error {
		return migtest.RestoreDB(tx, sessionBucketKey, sessionDBBefore)
	}

	// Put together what we expect the resulting index will look like.
	expectedIndex := map[string]interface{}{
		string(sess1ID[:]): map[string]interface{}{
			string(sessionKeyKey): string(sess1Key),
		},
		string(sess2ID[:]): map[string]interface{}{
			string(sessionKeyKey): string(sess2Key),
		},
		string(sess3ID[:]): map[string]interface{}{
			string(sessionKeyKey): string(sess3Key),
		},
	}

	sessionDBAfter := map[string]interface{}{
		string(sess1Key):   string(sess1Bytes),
		string(sess2Key):   string(sess2Bytes),
		string(sess3Key):   string(sess3Bytes),
		string(idIndexKey): expectedIndex,
	}

	// After the migration, we should have a new index bucket.
	after := func(tx *bbolt.Tx) error {
		return migtest.VerifyDB(tx, sessionBucketKey, sessionDBAfter)
	}

	migtest.ApplyMigration(
		t, before, after, MigrateSessionIDToKeyIndex, false,
	)
}

// newSession is a helper function that can be used to generate a random new
// Session. It returns the session ID, key and the serialised session.
func newSession(t *testing.T) (ID, []byte, []byte) {
	session, err := NewSession(
		"test-session", TypeMacaroonAdmin,
		time.Date(99999, 1, 1, 0, 0, 0, 0, time.UTC),
		"foo.bar.baz:1234", true, nil, nil, nil, false,
	)
	require.NoError(t, err)

	var sessBuff bytes.Buffer
	err = SerializeSession(&sessBuff, session)
	require.NoError(t, err)

	key := session.LocalPublicKey.SerializeCompressed()

	return session.ID, key, sessBuff.Bytes()
}

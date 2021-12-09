package session

import (
	"bytes"
	"testing"
	"time"

	"gopkg.in/macaroon-bakery.v2/bakery"

	"github.com/btcsuite/btcd/btcec"

	"github.com/stretchr/testify/require"
)

var (
	testRootKey  = []byte("54321")
	testID       = []byte("dummyId")
	testLocation = "lnd"
)

// TestSerializeDeserializeSession makes sure that a session can be serialized
// and deserialized from and to the tlv binary format successfully.
func TestSerializeDeserializeSession(t *testing.T) {
	perms := []bakery.Op{
		{
			Entity: "loop",
			Action: "read",
		},
		{
			Entity: "lnd",
			Action: "write",
		},
		{
			Entity: "pool",
			Action: "read",
		},
	}

	session, err := NewSession(
		"this is a session", TypeMacaroonCustom,
		time.Date(99999, 1, 1, 0, 0, 0, 0, time.UTC),
		"foo.bar.baz:1234", true, perms,
	)
	require.NoError(t, err)

	_, remotePubKey := btcec.PrivKeyFromBytes(btcec.S256(), testRootKey)
	session.RemotePublicKey = remotePubKey

	var buf bytes.Buffer
	require.NoError(t, SerializeSession(&buf, session))

	deserializedSession, err := DeserializeSession(&buf)
	require.NoError(t, err)

	// Apparently require.Equal doesn't work on time.Time when comparing as
	// a struct. So we need to compare the timestamp itself and then make
	// sure the rest of the session is equal separately.
	require.Equal(
		t, session.Expiry.Unix(), deserializedSession.Expiry.Unix(),
	)
	session.Expiry = time.Time{}
	deserializedSession.Expiry = time.Time{}
	require.Equal(t, session, deserializedSession)
}

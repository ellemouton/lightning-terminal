package session

import (
	"encoding/binary"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightninglabs/lightning-terminal/macaroons"
	"gopkg.in/macaroon.v2"
)

// Alias represents the id of a session.
type Alias [4]byte

// AliasFromMacaroon is a helper function that creates a session Alias from
// a macaroon Alias.
func AliasFromMacaroon(mac *macaroon.Macaroon) (Alias, error) {
	rootKeyID, err := macaroons.RootKeyIDFromMacaroon(mac)
	if err != nil {
		return Alias{}, err
	}

	return AliasFromMacRootKeyID(rootKeyID), nil
}

// AliasFromMacRootKeyID converts a macaroon root key Alias to a session Alias.
func AliasFromMacRootKeyID(rootKeyID uint64) Alias {
	rootKeyBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(rootKeyBytes[:], rootKeyID)

	var id Alias
	copy(id[:], rootKeyBytes[4:])

	return id
}

// AliasFromBytes is a helper function that creates a session Alias from a byte slice.
func AliasFromBytes(b []byte) (Alias, error) {
	var id Alias
	if len(b) != 4 {
		return id, fmt.Errorf("session Alias must be 4 bytes long")
	}
	copy(id[:], b)
	return id, nil
}

// NewSessionPrivKeyAndAlias randomly derives a new private key and session Alias
// pair.
func NewSessionPrivKeyAndAlias() (*btcec.PrivateKey, Alias, error) {
	var alias Alias

	privateKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, alias, fmt.Errorf("error deriving private key: %v",
			err)
	}

	pubKey := privateKey.PubKey()

	// NOTE: we use 4 bytes [1:5] of the serialised public key to create the
	// macaroon root key base along with the Session Alias. This will provide
	// 4 bytes of entropy. Previously, bytes [0:4] where used but this
	// resulted in lower entropy due to the first byte always being either
	// 0x02 or 0x03.
	copy(alias[:], pubKey.SerializeCompressed()[1:5])

	log.Debugf("Generated new Session Alias: %x", alias)

	return privateKey, alias, nil
}

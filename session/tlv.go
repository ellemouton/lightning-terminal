package session

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gopkg.in/macaroon-bakery.v2/bakery"

	"github.com/btcsuite/btcd/btcec"
	"github.com/lightningnetwork/lnd/tlv"
)

const (
	typeLabel           tlv.Type = 1
	typeState           tlv.Type = 2
	typeType            tlv.Type = 3
	typeExpiry          tlv.Type = 4
	typeServerAddr      tlv.Type = 5
	typeDevServer       tlv.Type = 6
	typeMacaroonRootKey tlv.Type = 7
	typePairingSecret   tlv.Type = 9
	typeLocalPrivateKey tlv.Type = 10
	typeRemotePublicKey tlv.Type = 11
	typeMacaroonRecipe  tlv.Type = 12

	// typeMacaroon is no longer used but we leave it defined for backwards
	// compatibility.
	typeMacaroon tlv.Type = 8
)

// SerializeSession binary serializes the given session to the writer using the
// tlv format.
func SerializeSession(w io.Writer, session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	var (
		label         = []byte(session.Label)
		state         = uint8(session.State)
		typ           = uint8(session.Type)
		expiry        = uint64(session.Expiry.Unix())
		serverAddr    = []byte(session.ServerAddr)
		devServer     = uint8(0)
		pairingSecret = session.PairingSecret[:]
		privateKey    = session.LocalPrivateKey.Serialize()
	)

	if session.DevServer {
		devServer = 1
	}

	tlvRecords := []tlv.Record{
		tlv.MakePrimitiveRecord(typeLabel, &label),
		tlv.MakePrimitiveRecord(typeState, &state),
		tlv.MakePrimitiveRecord(typeType, &typ),
		tlv.MakePrimitiveRecord(typeExpiry, &expiry),
		tlv.MakePrimitiveRecord(typeServerAddr, &serverAddr),
		tlv.MakePrimitiveRecord(typeDevServer, &devServer),
		tlv.MakePrimitiveRecord(
			typeMacaroonRootKey, &session.MacaroonRootKey,
		),
	}

	tlvRecords = append(
		tlvRecords,
		tlv.MakePrimitiveRecord(typePairingSecret, &pairingSecret),
		tlv.MakePrimitiveRecord(typeLocalPrivateKey, &privateKey),
	)

	if session.RemotePublicKey != nil {
		tlvRecords = append(tlvRecords, tlv.MakePrimitiveRecord(
			typeRemotePublicKey, &session.RemotePublicKey,
		))
	}

	if len(session.MacaroonRecipe) != 0 {
		macRecipe := macaroonPermsToString(session.MacaroonRecipe)
		macRecipeBytes := []byte(macRecipe)
		tlvRecords = append(tlvRecords, tlv.MakePrimitiveRecord(
			typeMacaroonRecipe, &macRecipeBytes),
		)
	}

	tlvStream, err := tlv.NewStream(tlvRecords...)
	if err != nil {
		return err
	}

	return tlvStream.Encode(w)
}

// DeserializeSession deserializes a session from the given reader, expecting
// the data to be encoded in the tlv format.
func DeserializeSession(r io.Reader) (*Session, error) {
	var (
		session                   = &Session{}
		label, serverAddr         []byte
		macRecipe                 []byte
		pairingSecret, privateKey []byte
		state, typ, devServer     uint8
		expiry                    uint64
	)
	tlvStream, err := tlv.NewStream(
		tlv.MakePrimitiveRecord(typeLabel, &label),
		tlv.MakePrimitiveRecord(typeState, &state),
		tlv.MakePrimitiveRecord(typeType, &typ),
		tlv.MakePrimitiveRecord(typeExpiry, &expiry),
		tlv.MakePrimitiveRecord(typeServerAddr, &serverAddr),
		tlv.MakePrimitiveRecord(typeDevServer, &devServer),
		tlv.MakePrimitiveRecord(
			typeMacaroonRootKey, &session.MacaroonRootKey,
		),
		tlv.MakePrimitiveRecord(typePairingSecret, &pairingSecret),
		tlv.MakePrimitiveRecord(typeLocalPrivateKey, &privateKey),
		tlv.MakePrimitiveRecord(
			typeRemotePublicKey, &session.RemotePublicKey,
		),
		tlv.MakePrimitiveRecord(typeMacaroonRecipe, &macRecipe),
	)
	if err != nil {
		return nil, err
	}

	parsedTypes, err := tlvStream.DecodeWithParsedTypes(r)
	if err != nil {
		return nil, err
	}

	session.Label = string(label)
	session.State = State(state)
	session.Type = Type(typ)
	session.Expiry = time.Unix(int64(expiry), 0)
	session.ServerAddr = string(serverAddr)
	session.DevServer = devServer == 1

	if t, ok := parsedTypes[typePairingSecret]; ok && t == nil {
		copy(session.PairingSecret[:], pairingSecret)
	}

	if t, ok := parsedTypes[typeLocalPrivateKey]; ok && t == nil {
		session.LocalPrivateKey, session.LocalPublicKey = btcec.PrivKeyFromBytes(
			btcec.S256(), privateKey,
		)
	}

	if t, ok := parsedTypes[typeMacaroonRecipe]; ok && t == nil {
		recipe, err := macaroonPermsFromString(string(macRecipe))
		if err != nil {
			return nil, err
		}

		session.MacaroonRecipe = recipe
	}

	return session, nil
}

func macaroonPermsToString(perms []bakery.Op) string {
	var macRecipe string
	for _, op := range perms {
		if macRecipe != "" {
			macRecipe += ";"
		}

		macRecipe += fmt.Sprintf("%s:%s", op.Entity, op.Action)
	}

	return macRecipe
}

func macaroonPermsFromString(macRecipe string) ([]bakery.Op, error) {
	perms := strings.Split(macRecipe, ";")
	recipe := make([]bakery.Op, 0, len(perms))
	for _, permStr := range perms {
		perm := strings.Split(permStr, ":")
		if len(perm) != 2 {
			return nil, errors.New("invalid macaroon " +
				"recipe encoding")
		}

		recipe = append(recipe, bakery.Op{
			Entity: perm[0],
			Action: perm[1],
		})
	}

	return recipe, nil
}

package session

import (
	"context"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightninglabs/lightning-node-connect/mailbox"
	"github.com/lightninglabs/lightning-terminal/macaroons"
	"gopkg.in/macaroon-bakery.v2/bakery"
	"gopkg.in/macaroon.v2"
)

// Type represents the type of session.
type Type uint8

const (
	TypeMacaroonReadonly Type = 0
	TypeMacaroonAdmin    Type = 1
	TypeMacaroonCustom   Type = 2
	TypeUIPassword       Type = 3
	TypeAutopilot        Type = 4
	TypeMacaroonAccount  Type = 5
)

// State represents the state of a session.
type State uint8

/*
				   /---> StateExpired (terminal)
StateReserved ---> StateCreated ---
       | 			  |
       	\-------------------------+---> StateRevoked (terminal)
*/

const (
	// StateCreated is the state of a session once it has been fully
	// committed to the BoltStore and is ready to be used. This is the
	// first state after StateReserved.
	StateCreated State = 0

	// StateInUse is the state of a session that is currently being used.
	//
	// NOTE: this state is not currently used, but we keep it around for now
	// since old sessions might still have this state persisted.
	StateInUse State = 1

	// StateRevoked is the state of a session that has been revoked before
	// its expiry date.
	StateRevoked State = 2

	// StateExpired is the state of a session that has passed its expiry
	// date.
	StateExpired State = 3

	// StateReserved is a temporary initial state of a session. This is used
	// to reserve a unique Alias and private key pair for a session before it
	// is fully created. On start-up, any sessions in this state should be
	// cleaned up.
	StateReserved State = 4
)

// Terminal returns true if the state is a terminal state.
func (s State) Terminal() bool {
	return s == StateExpired || s == StateRevoked
}

// legalStateShifts is a map that defines the legal State transitions that a
// Session can be put through.
var legalStateShifts = map[State]map[State]bool{
	StateReserved: {
		StateCreated: true,
		StateRevoked: true,
	},
	StateCreated: {
		StateExpired: true,
		StateRevoked: true,
	},
	StateInUse: {
		StateRevoked: true,
		StateExpired: true,
	},
}

// MacaroonRecipe defines the permissions and caveats that should be used
// to bake a macaroon.
type MacaroonRecipe struct {
	Permissions []bakery.Op
	Caveats     []macaroon.Caveat
}

// FeaturesConfig is a map from feature name to a raw byte array which stores
// any config feature config options.
type FeaturesConfig map[string][]byte

// Session is a struct representing a long-term Terminal Connect session.
type Session struct {
	Alias             Alias
	Label             string
	State             State
	Type              Type
	Expiry            time.Time
	CreatedAt         time.Time
	RevokedAt         time.Time
	ServerAddr        string
	DevServer         bool
	MacaroonRootKey   uint64
	MacaroonRecipe    *MacaroonRecipe
	PairingSecret     [mailbox.NumPassphraseEntropyBytes]byte
	LocalPrivateKey   *btcec.PrivateKey
	LocalPublicKey    *btcec.PublicKey
	RemotePublicKey   *btcec.PublicKey
	FeatureConfig     *FeaturesConfig
	WithPrivacyMapper bool
	PrivacyFlags      PrivacyFlags

	// GroupAlias is the Session Alias of the very first Session in the linked
	// group of sessions. If this is the very first session in the group
	// then this will be the same as Alias.
	GroupAlias Alias
}

// buildSession creates a new session with the given user-defined parameters.
func buildSession(alias Alias, localPrivKey *btcec.PrivateKey, label string, typ Type,
	created, expiry time.Time, serverAddr string, devServer bool,
	perms []bakery.Op, caveats []macaroon.Caveat,
	featureConfig FeaturesConfig, privacy bool, linkedGroupAlias *Alias,
	flags PrivacyFlags) (*Session, error) {

	_, pairingSecret, err := mailbox.NewPassphraseEntropy()
	if err != nil {
		return nil, fmt.Errorf("error deriving pairing secret: %v", err)
	}

	macRootKey := macaroons.NewSuperMacaroonRootKeyID(alias)

	// The group Alias will by default be the same as the Session Alias
	// unless this session links to a previous session.
	groupAlias := alias
	if linkedGroupAlias != nil {
		// If this session is linked to a previous session, then the
		// group Alias is the same as the linked session's group Alias.
		groupAlias = *linkedGroupAlias
	}

	sess := &Session{
		Alias:             alias,
		Label:             label,
		State:             StateReserved,
		Type:              typ,
		Expiry:            expiry.UTC(),
		CreatedAt:         created.UTC(),
		ServerAddr:        serverAddr,
		DevServer:         devServer,
		MacaroonRootKey:   macRootKey,
		PairingSecret:     pairingSecret,
		LocalPrivateKey:   localPrivKey,
		LocalPublicKey:    localPrivKey.PubKey(),
		RemotePublicKey:   nil,
		WithPrivacyMapper: privacy,
		PrivacyFlags:      flags,
		GroupAlias:        groupAlias,
	}

	if perms != nil || caveats != nil {
		sess.MacaroonRecipe = &MacaroonRecipe{
			Permissions: perms,
			Caveats:     caveats,
		}
	}

	if len(featureConfig) != 0 {
		sess.FeatureConfig = &featureConfig
	}

	return sess, nil
}

// AliasToGroupIndex defines an interface for the session Alias to group Alias
// index.
type AliasToGroupIndex interface {
	// GetGroupAlias will return the group Alias for the given session Alias.
	GetGroupAlias(ctx context.Context, sessionAlias Alias) (Alias, error)

	// GetSessionAliases will return the set of session Aliases that are in
	// the group with the given Alias.
	GetSessionAliases(ctx context.Context, groupAlias Alias) ([]Alias, error)
}

// Store is the interface a persistent storage must implement for storing and
// retrieving Terminal Connect sessions.
type Store interface {
	// NewSession creates a new session with the given user-defined
	// parameters. The session will remain in the StateReserved state until
	// ShiftState is called to update the state.
	NewSession(ctx context.Context, label string, typ Type,
		expiry time.Time, serverAddr string,
		devServer bool, perms []bakery.Op, caveats []macaroon.Caveat,
		featureConfig FeaturesConfig, privacy bool, linkedGroupID *Alias,
		flags PrivacyFlags) (*Session, error)

	// GetSession fetches the session with the given key.
	GetSession(ctx context.Context, key *btcec.PublicKey) (*Session, error)

	// ListAllSessions returns all sessions currently known to the store.
	ListAllSessions(ctx context.Context) ([]*Session, error)

	// ListSessionsByType returns all sessions of the given type.
	ListSessionsByType(ctx context.Context, t Type) ([]*Session, error)

	// ListSessionsByState returns all sessions currently known to the store
	// that are in the given states.
	ListSessionsByState(ctx context.Context, state State) ([]*Session,
		error)

	// UpdateSessionRemotePubKey can be used to add the given remote pub key
	// to the session with the given local pub key.
	UpdateSessionRemotePubKey(ctx context.Context, localPubKey,
		remotePubKey *btcec.PublicKey) error

	// GetSessionByAlias fetches the session with the given Alias.
	GetSessionByAlias(ctx context.Context, id Alias) (*Session, error)

	// DeleteReservedSessions deletes all sessions that are in the
	// StateReserved state.
	DeleteReservedSessions(ctx context.Context) error

	// ShiftState updates the state of the session with the given Alias to the
	// "dest" state.
	ShiftState(ctx context.Context, alias Alias, dest State) error

	AliasToGroupIndex
}

// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.25.0

package sqlc

import (
	"database/sql"
	"time"
)

type Account struct {
	ID                 int64
	Alias              int64
	Label              sql.NullString
	Type               int16
	InitialBalanceMsat int64
	CurrentBalanceMsat int64
	LastUpdated        time.Time
	Expiration         time.Time
}

type AccountIndex struct {
	Name  string
	Value int64
}

type AccountInvoice struct {
	AccountID int64
	Hash      []byte
}

type AccountPayment struct {
	AccountID      int64
	Hash           []byte
	Status         int16
	FullAmountMsat int64
}

type Session struct {
	ID              int64
	Alias           []byte
	Label           string
	State           int16
	Type            int16
	Expiry          time.Time
	CreatedAt       time.Time
	RevokedAt       sql.NullTime
	ServerAddress   string
	DevServer       bool
	MacaroonRootKey int64
	PairingSecret   []byte
	LocalPrivateKey []byte
	LocalPublicKey  []byte
	RemotePublicKey []byte
	Privacy         bool
	AccountID       sql.NullInt64
	GroupID         sql.NullInt64
}

type SessionFeatureConfig struct {
	SessionID   int64
	FeatureName string
	Config      []byte
}

type SessionMacaroonCaveat struct {
	SessionID      int64
	ID             []byte
	VerificationID []byte
	Location       sql.NullString
}

type SessionMacaroonPermission struct {
	SessionID int64
	Entity    string
	Action    string
}

type SessionPrivacyFlag struct {
	SessionID int64
	Flag      int32
}

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
	LegacyID           []byte
	Label              sql.NullString
	Type               int16
	IntialBalanceMsat  int64
	CurrentBalanceMsat int64
	LastUpdated        time.Time
	Expiration         time.Time
}

type AccountIndicy struct {
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

type Action struct {
	ID                 int64
	SessionID          sql.NullInt64
	AccountID          sql.NullInt64
	MacaroonIdentifier []byte
	ActorName          sql.NullString
	FeatureName        sql.NullString
	Trigger            sql.NullString
	Intent             sql.NullString
	StructuredJsonData sql.NullString
	RpcMethod          string
	RpcParamsJson      []byte
	CreatedAt          time.Time
	State              int16
	ErrorReason        sql.NullString
}

type FeatureConfig struct {
	SessionID   int64
	FeatureName string
	Config      []byte
}

type Kvstore struct {
	Perm        bool
	RuleName    string
	SessionID   sql.NullInt64
	FeatureName sql.NullString
	Key         string
	Value       []byte
}

type MacaroonCaveat struct {
	SessionID      int64
	ID             []byte
	VerificationID []byte
	Location       sql.NullString
}

type MacaroonPermission struct {
	SessionID int64
	Entity    string
	Action    string
}

type PrivacyFlag struct {
	SessionID int64
	Flag      int32
}

type PrivacyPair struct {
	GroupID int64
	Real    string
	Pseudo  string
}

type Session struct {
	ID              int64
	LegacyID        []byte
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
	GroupID         sql.NullInt64
}

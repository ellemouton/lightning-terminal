package firewalldb

import (
	"context"

	"github.com/lightninglabs/lightning-terminal/session"
)

// SessionDB is an interface that abstracts the database operations needed for
// the privacy mapper to function.
type SessionDB interface {
	session.AliasToGroupIndex

	// GetSessionByID returns the session for a specific id.
	GetSessionByAlias(context.Context, session.Alias) (*session.Session,
		error)
}

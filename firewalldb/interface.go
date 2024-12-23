package firewalldb

import (
	"context"

	"github.com/lightninglabs/lightning-terminal/session"
)

// SessionDB is an interface that abstracts the database operations needed for
// the privacy mapper to function.
type SessionDB interface {
	session.IDToGroupIndex

	// GetSessionByID returns the session for a specific id.
	GetSessionByID(context.Context, session.ID) (*session.Session, error)
}

type ActionDB interface {
	AddAction(ctx context.Context, action *Action) (uint64, error)

	SetActionState(ctx context.Context, al *ActionLocator,
		state ActionState, errReason string) error

	ListActions(ctx context.Context, filterFn ListActionsFilterFn,
		query *ListActionsQuery) ([]*Action, uint64, uint64, error)

	ListSessionActions(ctx context.Context, sessionID session.ID,
		filterFn ListActionsFilterFn, query *ListActionsQuery) (
		[]*Action, uint64, uint64, error)

	ListGroupActions(ctx context.Context, groupID session.ID,
		filterFn ListActionsFilterFn) ([]*Action, error)
}

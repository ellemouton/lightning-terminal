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

// ActionDB is an interface that abstracts the database operations needed for
// the Action persistence and querying.
type ActionDB interface {
	// AddAction persists the given action to the database.
	AddAction(ctx context.Context, action *Action) (uint64, error)

	// SetActionState finds the action specified by the ActionLocator and
	// sets its state to the given state.
	SetActionState(ctx context.Context, al *ActionLocator,
		state ActionState, errReason string) error

	// ListActions returns a list of Actions that pass the filterFn
	// requirements. The query IndexOffset and MaxNum params can be used to
	// control the number of actions returned. The return values are the
	// list of actions, the last index and the total count (iff
	// query.CountTotal is set).
	ListActions(ctx context.Context, query *ListActionsQuery,
		options ...ListActionOption) ([]*Action, uint64, uint64, error)

	// GetActionsReadDB produces an ActionReadDB using the given group ID
	// and feature name.
	GetActionsReadDB(groupID session.ID, featureName string) ActionsReadDB
}

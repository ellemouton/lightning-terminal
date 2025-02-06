package session

import "errors"

var (
	// ErrSessionNotFound is an error returned when we attempt to retrieve
	// information about a session but it is not found.
	ErrSessionNotFound = errors.New("session not found")

	// ErrSessionExists is returned when an attempt is made to insert a new
	// session that collides with an existing session's unique fields such
	// as local static key.
	ErrSessionExists = errors.New("session already exists")

	// ErrUnknownGroup is returned when an attempt is made to insert a
	// session and link it to an existing group where the group is not
	// known.
	ErrUnknownGroup = errors.New("unknown group")

	// ErrSessionsInGroupStillActive is returned when an attempt is made to
	// insert a session and link it to a group that still has other active
	// sessions.
	ErrSessionsInGroupStillActive = errors.New(
		"group has active sessions",
	)
)

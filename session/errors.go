package session

import (
	"errors"
)

var (
	// ErrSessionExists is returned when an attempt is made to insert a new
	// session that collides with an existing session's unique fields such
	// as local static key.
	ErrSessionExists = errors.New("session already exists")

	ErrUnknownLinkedSession = errors.New("unknown linked session")

	ErrSessionsInGroupStillActive = errors.New("sessions in group still " +
		"active")

	ErrSessionUnknown = errors.New("session unknown")

	ErrGroupUnknown = errors.New("group unknown")
)

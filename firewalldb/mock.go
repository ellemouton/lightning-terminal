package firewalldb

import (
	"context"
	"fmt"

	"github.com/lightninglabs/lightning-terminal/session"
)

type mockSessionDB struct {
	sessionToGroupID  map[session.Alias]session.Alias
	groupToSessionIDs map[session.Alias][]session.Alias
	privacyFlags      map[session.Alias]session.PrivacyFlags
}

var _ SessionDB = (*mockSessionDB)(nil)

// NewMockSessionDB creates a new mock privacy map details instance.
func NewMockSessionDB() *mockSessionDB {
	return &mockSessionDB{
		sessionToGroupID:  make(map[session.Alias]session.Alias),
		groupToSessionIDs: make(map[session.Alias][]session.Alias),
		privacyFlags:      make(map[session.Alias]session.PrivacyFlags),
	}
}

// AddPair adds a new session to group Alias pair to the mock details.
func (m *mockSessionDB) AddPair(sessionID, groupID session.Alias) {
	m.sessionToGroupID[sessionID] = groupID

	m.groupToSessionIDs[groupID] = append(
		m.groupToSessionIDs[groupID], sessionID,
	)
}

// GetGroupID returns the group Alias for the given session Alias.
func (m *mockSessionDB) GetGroupAlias(_ context.Context, sessionID session.Alias) (
	session.Alias, error) {

	id, ok := m.sessionToGroupID[sessionID]
	if !ok {
		return session.Alias{}, fmt.Errorf("no group Alias found for " +
			"session Alias")
	}

	return id, nil
}

// GetSessionIDs returns the set of session IDs that are in the group
func (m *mockSessionDB) GetSessionAliases(_ context.Context, groupID session.Alias) (
	[]session.Alias, error) {

	ids, ok := m.groupToSessionIDs[groupID]
	if !ok {
		return nil, fmt.Errorf("no session IDs found for group Alias")
	}

	return ids, nil
}

// GetSessionByID returns the session for a specific id.
func (m *mockSessionDB) GetSessionByAlias(_ context.Context,
	sessionID session.Alias) (*session.Session, error) {

	s, ok := m.sessionToGroupID[sessionID]
	if !ok {
		return nil, fmt.Errorf("no session found for session Alias")
	}

	f, ok := m.privacyFlags[sessionID]
	if !ok {
		return nil, fmt.Errorf("no privacy flags found for session Alias")
	}

	return &session.Session{GroupAlias: s, PrivacyFlags: f}, nil
}

// AddPrivacyFlags is a helper that adds privacy flags to the mock session db.
func (m *mockSessionDB) AddPrivacyFlags(sessionID session.Alias,
	flags session.PrivacyFlags) error {

	m.privacyFlags[sessionID] = flags

	return nil
}

package firewalldb

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/lightninglabs/lightning-terminal/accounts"
	"github.com/lightninglabs/lightning-terminal/db"
	"github.com/lightninglabs/lightning-terminal/session"
	"github.com/stretchr/testify/require"
)

type testActionDB struct {
	ActionDB
	session.Store
}

// TestActionsDB tests the basic functionality of the ActionsDB.
func TestActionsDB(t *testing.T) {
	testList := []struct {
		name string
		test func(t *testing.T,
			makeDB func(t *testing.T) testActionDB)
	}{
		{
			name: "BasicActionStore",
			test: testActionStorage,
		},
		{
			name: "ListActions",
			test: testListActions,
		},
		{
			name: "ListGroupActions",
			test: testListGroupActions,
		},
	}

	makeKeyValueDBs := func(t *testing.T) testActionDB {
		tempDir := t.TempDir()

		sessionsDB, err := session.NewDB(tempDir, "test_sessions.db")
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, sessionsDB.Close())
		})

		accountsDB, err := accounts.NewBoltStore(
			tempDir, "test_accounts.db",
		)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, accountsDB.Close())
		})

		actionDB, err := NewDB(
			tempDir, "test_actions.db", sessionsDB, accountsDB,
		)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, actionDB.Close())
		})

		return testActionDB{
			ActionDB: actionDB,
			Store:    sessionsDB,
		}
	}

	// First create a shared Postgres instance so we don't spawn a new
	// docker container for each test.
	pgFixture := db.NewTestPgFixture(
		t, db.DefaultPostgresFixtureLifetime,
	)
	t.Cleanup(func() {
		pgFixture.TearDown(t)
	})

	makeSQLDB := func(t *testing.T, sqlite bool) testActionDB {
		var sqlDB *db.BaseDB
		if sqlite {
			sqlDB = db.NewTestSqliteDB(t).BaseDB
		} else {
			sqlDB = db.NewTestPostgresDB(t, pgFixture).BaseDB
		}

		sessionsExecutor := db.NewTransactionExecutor(
			sqlDB, func(tx *sql.Tx) session.SQLQueries {
				return sqlDB.WithTx(tx)
			},
		)

		actionsExecutor := db.NewTransactionExecutor(
			sqlDB, func(tx *sql.Tx) SQLActionQueries {
				return sqlDB.WithTx(tx)
			},
		)

		return testActionDB{
			ActionDB: NewSQLActionsStore(actionsExecutor),
			Store:    session.NewSQLStore(sessionsExecutor),
		}
	}

	for _, test := range testList {
		t.Run(test.name+"_KV", func(t *testing.T) {
			test.test(t, makeKeyValueDBs)
		})

		// TODO(elle): fix sqlite time stamp issue.
		//t.Run(test.name+"_SQLite", func(t *testing.T) {
		//	test.test(t, func(t *testing.T) Store {
		//		return makeSQLDB(t, true)
		//	})
		//})

		t.Run(test.name+"_Postgres", func(t *testing.T) {
			test.test(t, func(t *testing.T) testActionDB {
				return makeSQLDB(t, false)
			})
		})
	}
}

// testActionStorage tests that the ActionsListDB CRUD logic.
func testActionStorage(t *testing.T, makeDB func(t *testing.T) testActionDB) {
	t.Parallel()

	ctx := context.Background()
	db := makeDB(t)

	session1 := newSession(t, db.Store, nil)
	session2 := newSession(t, db.Store, nil)
	action1 := makeAction(session1.ID)
	action2 := makeAction(session2.ID)

	// If the session does not yet exist, then we expect an error if we
	// list actions by session ID.
	_, _, _, err := db.ListActions(
		ctx, nil,
		WithActionSessionID(session1.ID),
		WithActionState(ActionStateDone),
	)
	require.ErrorIs(t, err, session.ErrSessionUnknown)

	//// Adding an action for a session that does not exist yet also results
	//// in an error.
	//action1Locator, err := db.AddAction(ctx, action1)
	//require.ErrorIs(t, err, session.ErrSessionUnknown)

	// Insert a sessions 1 and 2.
	require.NoError(t, db.CreateSession(ctx, session1))
	require.NoError(t, db.CreateSession(ctx, session2))

	actions, _, _, err := db.ListActions(
		ctx, nil,
		WithActionSessionID(session1.ID),
		WithActionState(ActionStateDone),
	)
	require.NoError(t, err)
	require.Len(t, actions, 0)

	actions, _, _, err = db.ListActions(
		ctx, nil,
		WithActionSessionID(session2.ID),
		WithActionState(ActionStateDone),
	)
	require.NoError(t, err)
	require.Len(t, actions, 0)

	action1Locator, err := db.AddAction(ctx, action1)
	require.NoError(t, err)
	err = db.SetActionState(ctx, action1Locator, ActionStateDone, "")
	require.NoError(t, err)

	locator2, err := db.AddAction(ctx, action2)
	require.NoError(t, err)

	actions, _, _, err = db.ListActions(
		ctx, nil,
		WithActionSessionID(session1.ID),
		WithActionState(ActionStateDone),
	)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	assertEqualActions(t, action1, ActionStateDone, "", actions[0])

	actions, _, _, err = db.ListActions(
		ctx, nil,
		WithActionSessionID(session2.ID),
		WithActionState(ActionStateDone),
	)
	require.NoError(t, err)
	require.Len(t, actions, 0)

	err = db.SetActionState(ctx, locator2, ActionStateDone, "")
	require.NoError(t, err)

	actions, _, _, err = db.ListActions(
		ctx, nil,
		WithActionSessionID(session2.ID),
		WithActionState(ActionStateDone),
	)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	assertEqualActions(t, action2, ActionStateDone, "", actions[0])

	_, err = db.AddAction(ctx, action1)
	require.NoError(t, err)

	// Check that providing no session id and no filter function returns
	// all the actions.
	actions, _, _, err = db.ListActions(ctx, &ListActionsQuery{
		IndexOffset: 0,
		MaxNum:      100,
		Reversed:    false,
	})
	require.NoError(t, err)
	require.Len(t, actions, 3)

	// Try set an error reason for a non Errored state.
	err = db.SetActionState(ctx, locator2, ActionStateDone, "hello")
	require.Error(t, err)

	// Now try move the action to errored with a reason.
	err = db.SetActionState(ctx, locator2, ActionStateError, "fail whale")
	require.NoError(t, err)

	actions, _, _, err = db.ListActions(
		ctx, nil,
		WithActionSessionID(session2.ID),
		WithActionState(ActionStateError),
	)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	assertEqualActions(t, action2, ActionStateError, "fail whale", actions[0])
}

// testListActions tests some ListAction options.
// TODO(elle): cover more test cases here.
func testListActions(t *testing.T, makeDB func(t *testing.T) testActionDB) {
	t.Parallel()
	ctx := context.Background()
	db := makeDB(t)

	var (
		session1   = newSession(t, db.Store, nil)
		session2   = newSession(t, db.Store, nil)
		sessionID1 = session1.ID
		sessionID2 = session2.ID
	)

	require.NoError(t, db.CreateSession(ctx, session1))
	require.NoError(t, db.CreateSession(ctx, session2))

	actionIds := 0
	addAction := func(sessionID [4]byte) {
		actionIds++
		req := &AddActionReq{
			MacaroonIdentifier: sessionID,
			ActorName:          "Autopilot",
			FeatureName:        fmt.Sprintf("%d", actionIds),
			Trigger:            "fee too low",
			Intent:             "increase fee",
			StructuredJsonData: "{\"something\":\"nothing\"}",
			RPCMethod:          "UpdateChanPolicy",
			RPCParamsJson:      []byte("new fee"),
		}

		al, err := db.AddAction(ctx, req)
		require.NoError(t, err)

		err = db.SetActionState(ctx, al, ActionStateDone, "")
		require.NoError(t, err)
	}

	type action struct {
		sessionID [4]byte
		actionID  string
	}

	assertActions := func(dbActions []*Action, al []*action) {
		require.Len(t, dbActions, len(al))
		for i, a := range al {
			require.True(t, dbActions[i].SessionID.IsSome())
			dbActions[i].SessionID.WhenSome(func(id session.ID) {
				require.EqualValues(
					t, a.sessionID, id,
				)
			})
			require.Equal(t, a.actionID, dbActions[i].FeatureName)
		}
	}

	addAction(sessionID1)
	addAction(sessionID1)
	addAction(sessionID1)
	addAction(sessionID1)
	addAction(sessionID2)

	actions, lastIndex, totalCount, err := db.ListActions(ctx, nil)
	require.NoError(t, err)
	require.Len(t, actions, 5)
	require.EqualValues(t, 5, lastIndex)
	require.EqualValues(t, 0, totalCount)
	assertActions(actions, []*action{
		{sessionID1, "1"},
		{sessionID1, "2"},
		{sessionID1, "3"},
		{sessionID1, "4"},
		{sessionID2, "5"},
	})

	query := &ListActionsQuery{
		Reversed: true,
	}

	actions, lastIndex, totalCount, err = db.ListActions(ctx, query)
	require.NoError(t, err)
	require.Len(t, actions, 5)
	require.EqualValues(t, 1, lastIndex)
	require.EqualValues(t, 0, totalCount)
	assertActions(actions, []*action{
		{sessionID2, "5"},
		{sessionID1, "4"},
		{sessionID1, "3"},
		{sessionID1, "2"},
		{sessionID1, "1"},
	})

	actions, lastIndex, totalCount, err = db.ListActions(
		ctx, &ListActionsQuery{
			CountAll: true,
		},
	)
	require.NoError(t, err)
	require.Len(t, actions, 5)
	require.EqualValues(t, 5, lastIndex)
	require.EqualValues(t, 5, totalCount)
	assertActions(actions, []*action{
		{sessionID1, "1"},
		{sessionID1, "2"},
		{sessionID1, "3"},
		{sessionID1, "4"},
		{sessionID2, "5"},
	})

	actions, lastIndex, totalCount, err = db.ListActions(
		ctx, &ListActionsQuery{
			CountAll: true,
			Reversed: true,
		},
	)
	require.NoError(t, err)
	require.Len(t, actions, 5)
	require.EqualValues(t, 1, lastIndex)
	require.EqualValues(t, 5, totalCount)
	assertActions(actions, []*action{
		{sessionID2, "5"},
		{sessionID1, "4"},
		{sessionID1, "3"},
		{sessionID1, "2"},
		{sessionID1, "1"},
	})

	addAction(sessionID2)
	addAction(sessionID2)
	addAction(sessionID1)
	addAction(sessionID1)
	addAction(sessionID2)

	actions, lastIndex, totalCount, err = db.ListActions(ctx, nil)
	require.NoError(t, err)
	require.Len(t, actions, 10)
	require.EqualValues(t, 10, lastIndex)
	require.EqualValues(t, 0, totalCount)
	assertActions(actions, []*action{
		{sessionID1, "1"},
		{sessionID1, "2"},
		{sessionID1, "3"},
		{sessionID1, "4"},
		{sessionID2, "5"},
		{sessionID2, "6"},
		{sessionID2, "7"},
		{sessionID1, "8"},
		{sessionID1, "9"},
		{sessionID2, "10"},
	})

	actions, lastIndex, totalCount, err = db.ListActions(
		ctx, &ListActionsQuery{
			MaxNum:   3,
			CountAll: true,
		},
	)
	require.NoError(t, err)
	require.Len(t, actions, 3)
	require.EqualValues(t, 3, lastIndex)
	require.EqualValues(t, 10, totalCount)
	assertActions(actions, []*action{
		{sessionID1, "1"},
		{sessionID1, "2"},
		{sessionID1, "3"},
	})

	actions, lastIndex, totalCount, err = db.ListActions(
		ctx, &ListActionsQuery{
			MaxNum:      3,
			IndexOffset: 3,
		},
	)
	require.NoError(t, err)
	require.Len(t, actions, 3)
	require.EqualValues(t, 6, lastIndex)
	require.EqualValues(t, 0, totalCount)
	assertActions(actions, []*action{
		{sessionID1, "4"},
		{sessionID2, "5"},
		{sessionID2, "6"},
	})

	actions, lastIndex, totalCount, err = db.ListActions(
		ctx, &ListActionsQuery{
			MaxNum:      3,
			IndexOffset: 3,
			CountAll:    true,
		},
	)
	require.NoError(t, err)
	require.Len(t, actions, 3)
	require.EqualValues(t, 6, lastIndex)
	require.EqualValues(t, 10, totalCount)
	assertActions(actions, []*action{
		{sessionID1, "4"},
		{sessionID2, "5"},
		{sessionID2, "6"},
	})
}

// testListGroupActions tests that the listGroupActions correctly returns all
// actions in a particular session group.
func testListGroupActions(t *testing.T, makeDB func(t *testing.T) testActionDB) {
	t.Parallel()
	ctx := context.Background()
	db := makeDB(t)

	// The given group does not yet exist, so ListActions should fail.
	_, _, _, err := db.ListActions(
		ctx, nil, WithActionGroupID(intToSessionID(1)),
	)
	require.ErrorIs(t, err, session.ErrUnknownGroup)

	// Link session 1 and session 2 to group 1 and persist them.
	session1 := newSession(t, db.Store, nil)
	session2 := newSession(t, db.Store, &session1.ID)
	require.NoError(t, db.CreateSession(ctx, session1))

	group1 := session1.GroupID

	// There should not be any actions in group 1 yet.
	al, _, _, err := db.ListActions(ctx, nil, WithActionGroupID(group1))
	require.NoError(t, err)
	require.Empty(t, al)

	// Add an action under session 1.
	action1 := makeAction(session1.ID)
	al1, err := db.AddAction(ctx, action1)
	require.NoError(t, err)
	require.NoError(t, db.SetActionState(ctx, al1, ActionStateDone, ""))

	// There should now be one action in the group.
	al, _, _, err = db.ListActions(ctx, nil, WithActionGroupID(group1))
	require.NoError(t, err)
	require.Len(t, al, 1)
	require.True(t, al[0].SessionID.IsSome())
	al[0].SessionID.WhenSome(func(id session.ID) {
		require.Equal(t, session1.ID, id)
	})

	// First revoke the first session before persisting the linked session.
	require.NoError(t, db.RevokeSession(ctx, session1.LocalPublicKey))
	require.NoError(t, db.CreateSession(ctx, session2))

	// Add an action under session 2.
	action2 := makeAction(session2.ID)
	_, err = db.AddAction(ctx, action2)
	require.NoError(t, err)

	// There should now be actions in the group.
	al, _, _, err = db.ListActions(ctx, nil, WithActionGroupID(group1))
	require.NoError(t, err)
	require.Len(t, al, 2)
	require.True(t, al[0].SessionID.IsSome())
	al[0].SessionID.WhenSome(func(id session.ID) {
		require.Equal(t, session1.ID, id)
	})
	require.True(t, al[1].SessionID.IsSome())
	al[1].SessionID.WhenSome(func(id session.ID) {
		require.Equal(t, session2.ID, id)
	})
}

func newSession(t *testing.T, db session.Store,
	linkedGroupID *session.ID) *session.Session {

	id, priv, err := db.GetUnusedIDAndKeyPair(context.Background())
	require.NoError(t, err)

	session, err := session.NewSession(
		id, priv, "", session.TypeMacaroonAdmin,
		time.Date(99999, 1, 1, 0, 0, 0, 0, time.UTC),
		"foo.bar.baz:1234", true, nil, nil, nil, true, linkedGroupID,
		[]session.PrivacyFlag{session.ClearPubkeys},
	)
	require.NoError(t, err)

	return session
}

func makeAction(sessionID session.ID) *AddActionReq {
	return &AddActionReq{
		MacaroonIdentifier: sessionID,
		ActorName:          "Autopilot",
		FeatureName:        "auto-fees",
		Trigger:            "fee too low",
		Intent:             "increase fee",
		StructuredJsonData: "{\"something\":\"nothing\"}",
		RPCMethod:          "UpdateChanPolicy",
		RPCParamsJson:      []byte("new fee"),
	}
}

func assertEqualActions(t *testing.T, expectedReq *AddActionReq,
	expectedState ActionState, expectedErr string, got *Action) {
	//require.Equal(t, expected.AttemptedAt.Unix(), got.AttemptedAt.Unix())
	//got.AttemptedAt = expected.AttemptedAt
	//require.Equal(t, expected, got)

	require.Equal(t, *expectedReq, got.AddActionReq)
	require.Equal(t, expectedState, got.State)
	require.Equal(t, expectedErr, got.ErrorReason)
}

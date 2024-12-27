package firewalldb

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	sessionID1 = intToSessionID(1)
	sessionID2 = intToSessionID(2)

	action1 = &Action{
		SessionID:          sessionID1,
		ActorName:          "Autopilot",
		FeatureName:        "auto-fees",
		Trigger:            "fee too low",
		Intent:             "increase fee",
		StructuredJsonData: "{\"something\":\"nothing\"}",
		RPCMethod:          "UpdateChanPolicy",
		RPCParamsJson:      []byte("new fee"),
		AttemptedAt:        time.Unix(32100, 0),
		State:              ActionStateDone,
	}

	action2 = &Action{
		SessionID:     sessionID2,
		ActorName:     "Autopilot",
		FeatureName:   "rebalancer",
		Trigger:       "channels not balanced",
		Intent:        "balance",
		RPCMethod:     "SendToRoute",
		RPCParamsJson: []byte("hops, amount"),
		AttemptedAt:   time.Unix(12300, 0),
		State:         ActionStateInit,
	}
)

// TestActionsDB tests the basic functionality of the ActionsDB.
func TestActionsDB(t *testing.T) {
	testList := []struct {
		name string
		test func(t *testing.T, makeDB func(t *testing.T) ActionDB)
	}{
		{
			name: "BasicSessionStore",
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

	makeKeyValueDB := func(t *testing.T) ActionDB {
		kvdb, err := NewDB(t.TempDir(), "test.db", nil)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, kvdb.Close())
		})

		return kvdb
	}

	for _, test := range testList {
		test := test
		t.Run(test.name+"_KV", func(t *testing.T) {
			test.test(t, makeKeyValueDB)
		})
	}
}

// testActionStorage tests that the ActionsListDB CRUD logic.
func testActionStorage(t *testing.T, makeDB func(t *testing.T) ActionDB) {
	ctx := context.Background()
	db := makeDB(t)

	actions, _, _, err := db.ListActions(
		ctx, nil,
		WithActionSessionID(sessionID1),
		WithActionState(ActionStateDone),
	)
	require.NoError(t, err)
	require.Len(t, actions, 0)

	actions, _, _, err = db.ListActions(
		ctx, nil,
		WithActionSessionID(sessionID2),
		WithActionState(ActionStateDone),
	)
	require.NoError(t, err)
	require.Len(t, actions, 0)

	_, err = db.AddAction(ctx, action1)
	require.NoError(t, err)

	locator2, err := db.AddAction(ctx, action2)
	require.NoError(t, err)

	actions, _, _, err = db.ListActions(
		ctx, nil,
		WithActionSessionID(sessionID1),
		WithActionState(ActionStateDone),
	)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	assertEqualActions(t, action1, actions[0])

	actions, _, _, err = db.ListActions(
		ctx, nil,
		WithActionSessionID(sessionID2),
		WithActionState(ActionStateDone),
	)
	require.NoError(t, err)
	require.Len(t, actions, 0)

	err = db.SetActionState(ctx, locator2, ActionStateDone, "")
	require.NoError(t, err)

	actions, _, _, err = db.ListActions(
		ctx, nil,
		WithActionSessionID(sessionID2),
		WithActionState(ActionStateDone),
	)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	action2.State = ActionStateDone
	assertEqualActions(t, action2, actions[0])

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
		WithActionSessionID(sessionID2),
		WithActionState(ActionStateError),
	)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	action2.State = ActionStateError
	action2.ErrorReason = "fail whale"
	assertEqualActions(t, action2, actions[0])
}

// testListActions tests some ListAction options.
// TODO(elle): cover more test cases here.
func testListActions(t *testing.T, makeDB func(t *testing.T) ActionDB) {
	t.Parallel()
	ctx := context.Background()
	db := makeDB(t)

	sessionID1 := [4]byte{1, 1, 1, 1}
	sessionID2 := [4]byte{2, 2, 2, 2}

	actionIds := 0
	addAction := func(sessionID [4]byte) {
		actionIds++
		action := &Action{
			SessionID:          sessionID,
			ActorName:          "Autopilot",
			FeatureName:        fmt.Sprintf("%d", actionIds),
			Trigger:            "fee too low",
			Intent:             "increase fee",
			StructuredJsonData: "{\"something\":\"nothing\"}",
			RPCMethod:          "UpdateChanPolicy",
			RPCParamsJson:      []byte("new fee"),
			AttemptedAt:        time.Unix(32100, 0),
			State:              ActionStateDone,
		}

		_, err := db.AddAction(ctx, action)
		require.NoError(t, err)
	}

	type action struct {
		sessionID [4]byte
		actionID  string
	}

	assertActions := func(dbActions []*Action, al []*action) {
		require.Len(t, dbActions, len(al))
		for i, a := range al {
			require.EqualValues(
				t, a.sessionID, dbActions[i].SessionID,
			)
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
func testListGroupActions(t *testing.T, makeDB func(t *testing.T) ActionDB) {
	t.Parallel()
	ctx := context.Background()
	db := makeDB(t)

	group1 := intToSessionID(0)

	// There should not be any actions in group 1 yet.
	al, _, _, err := db.ListActions(ctx, nil, WithActionGroupID(group1))
	require.NoError(t, err)
	require.Empty(t, al)

	// Add an action under session 1.
	_, err = db.AddAction(ctx, action1)
	require.NoError(t, err)

	// There should now be one action in the group.
	al, _, _, err = db.ListActions(ctx, nil, WithActionGroupID(group1))
	require.NoError(t, err)
	require.Len(t, al, 1)
	require.Equal(t, sessionID1, al[0].SessionID)

	// Add an action under session 2.
	_, err = db.AddAction(ctx, action2)
	require.NoError(t, err)

	// There should now be actions in the group.
	al, _, _, err = db.ListActions(ctx, nil, WithActionGroupID(group1))
	require.NoError(t, err)
	require.Len(t, al, 2)
	require.Equal(t, sessionID1, al[0].SessionID)
	require.Equal(t, sessionID2, al[1].SessionID)
}

func assertEqualActions(t *testing.T, expected, got *Action) {
	require.Equal(t, expected.AttemptedAt.Unix(), got.AttemptedAt.Unix())
	got.AttemptedAt = expected.AttemptedAt
	require.Equal(t, expected, got)
}

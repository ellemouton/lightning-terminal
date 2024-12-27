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

// TestActionStorage tests that the ActionsListDB CRUD logic.
func TestActionStorage(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	db, err := NewDB(tmpDir, "test.db", nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

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

	id, err := db.AddAction(ctx, action1)
	require.NoError(t, err)
	require.Equal(t, uint64(1), id)

	id, err = db.AddAction(ctx, action2)
	require.NoError(t, err)
	require.Equal(t, uint64(1), id)

	actions, _, _, err = db.ListActions(
		ctx, nil,
		WithActionSessionID(sessionID1),
		WithActionState(ActionStateDone),
	)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	require.Equal(t, action1, actions[0])

	actions, _, _, err = db.ListActions(
		ctx, nil,
		WithActionSessionID(sessionID2),
		WithActionState(ActionStateDone),
	)
	require.NoError(t, err)
	require.Len(t, actions, 0)

	err = db.SetActionState(
		ctx, &ActionLocator{
			SessionID: sessionID2,
			ActionID:  uint64(1),
		}, ActionStateDone, "",
	)
	require.NoError(t, err)

	actions, _, _, err = db.ListActions(
		ctx, nil,
		WithActionSessionID(sessionID2),
		WithActionState(ActionStateDone),
	)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	action2.State = ActionStateDone
	require.Equal(t, action2, actions[0])

	id, err = db.AddAction(ctx, action1)
	require.NoError(t, err)
	require.Equal(t, uint64(2), id)

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
	err = db.SetActionState(
		ctx, &ActionLocator{
			SessionID: sessionID2,
			ActionID:  uint64(1),
		}, ActionStateDone, "hello",
	)
	require.Error(t, err)

	// Now try move the action to errored with a reason.
	err = db.SetActionState(
		ctx, &ActionLocator{
			SessionID: sessionID2,
			ActionID:  uint64(1),
		}, ActionStateError, "fail whale",
	)
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
	require.Equal(t, action2, actions[0])
}

// TestListActions tests some ListAction options.
// TODO(elle): cover more test cases here.
func TestListActions(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	db, err := NewDB(tmpDir, "test.db", nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

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

// TestListGroupActions tests that the ListGroupActions correctly returns all
// actions in a particular session group.
func TestListGroupActions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	group1 := intToSessionID(0)

	// Link session 1 and session 2 to group 1.
	index := NewMockSessionDB()
	index.AddPair(sessionID1, group1)
	index.AddPair(sessionID2, group1)

	db, err := NewDB(t.TempDir(), "test.db", index)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

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

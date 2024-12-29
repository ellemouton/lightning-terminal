package firewalldb

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/lightninglabs/lightning-terminal/accounts"
	"github.com/lightninglabs/lightning-terminal/session"
	"github.com/lightninglabs/taproot-assets/fn"
	"github.com/lightningnetwork/lnd/tlv"
	"go.etcd.io/bbolt"
)

const (
	typeActorName          tlv.Type = 1
	typeFeature            tlv.Type = 2
	typeTrigger            tlv.Type = 3
	typeIntent             tlv.Type = 4
	typeStructuredJsonData tlv.Type = 5
	typeRPCMethod          tlv.Type = 6
	typeRPCParamsJson      tlv.Type = 7
	typeAttemptedAt        tlv.Type = 8
	typeState              tlv.Type = 9
	typeErrorReason        tlv.Type = 10

	typeLocatorSessionID tlv.Type = 1
	typeLocatorActionID  tlv.Type = 2
)

/*
	The Actions are stored in the following structure in the KV db:

	actions-bucket -> actions -> <session-id> -> <action-index> -> serialised action

		       -> actions-index -> <id> -> {sessionID:action-index}
*/

var (
	// actionsBucketKey is the key that will be used for the main Actions
	// bucket.
	actionsBucketKey = []byte("actions-bucket")

	// actionsKey is the key used for the sub-bucket containing the
	// session actions.
	actionsKey = []byte("actions")

	// actionsIndex is the key used for the sub-bucket containing a map
	// from monotonically increasing IDs to action locators.
	actionsIndex = []byte("actions-index")
)

// AddAction serialises and adds an Action to the DB under the given sessionID.
//
// NOTE: This is part of the ActionDB interface.
func (db *DB) AddAction(ctx context.Context, req *AddActionReq) (ActionLocator,
	error) {

	// TODO(elle): NOTE: initially it would error if we did not find the session or
	// account to link it to. but now if it cant link them, it will just
	// store the mac identifier. So coupling is not enforced. so no error
	// here. So really no need to check here.

	action := &Action{
		AttemptedAt:  time.Now(),
		State:        ActionStateInit,
		AddActionReq: *req,
	}

	var buf bytes.Buffer
	if err := SerializeAction(&buf, action); err != nil {
		return nil, err
	}

	var locator kvdbActionLocator
	err := db.DB.Update(func(tx *bbolt.Tx) error {
		mainActionsBucket, err := getBucket(tx, actionsBucketKey)
		if err != nil {
			return err
		}

		actionsBucket := mainActionsBucket.Bucket(actionsKey)
		if actionsBucket == nil {
			return ErrNoSuchKeyFound
		}

		sessBucket, err := actionsBucket.CreateBucketIfNotExists(
			action.MacaroonIdentifier[:],
		)
		if err != nil {
			return err
		}

		nextActionIndex, err := sessBucket.NextSequence()
		if err != nil {
			return err
		}

		var actionIndex [8]byte
		byteOrder.PutUint64(actionIndex[:], nextActionIndex)
		err = sessBucket.Put(actionIndex[:], buf.Bytes())
		if err != nil {
			return err
		}

		actionsIndexBucket := mainActionsBucket.Bucket(actionsIndex)
		if actionsIndexBucket == nil {
			return ErrNoSuchKeyFound
		}

		nextSeq, err := actionsIndexBucket.NextSequence()
		if err != nil {
			return err
		}

		locator = kvdbActionLocator{
			SessionID: action.MacaroonIdentifier,
			ActionID:  nextActionIndex,
		}

		var buf bytes.Buffer
		err = serializeActionLocator(&buf, &locator)
		if err != nil {
			return err
		}

		var seqNoBytes [8]byte
		byteOrder.PutUint64(seqNoBytes[:], nextSeq)
		return actionsIndexBucket.Put(seqNoBytes[:], buf.Bytes())
	})
	if err != nil {
		return nil, err
	}

	return &locator, nil
}

func putAction(tx *bbolt.Tx, al *kvdbActionLocator, a *Action) error {
	var buf bytes.Buffer
	if err := SerializeAction(&buf, a); err != nil {
		return err
	}

	mainActionsBucket, err := getBucket(tx, actionsBucketKey)
	if err != nil {
		return err
	}

	actionsBucket := mainActionsBucket.Bucket(actionsKey)
	if actionsBucket == nil {
		return ErrNoSuchKeyFound
	}

	sessBucket := actionsBucket.Bucket(al.SessionID[:])
	if sessBucket == nil {
		return fmt.Errorf("session bucket for session ID %x does not "+
			"exist", al.SessionID)
	}

	var id [8]byte
	binary.BigEndian.PutUint64(id[:], al.ActionID)

	return sessBucket.Put(id[:], buf.Bytes())
}

func (db *DB) getAction(ctx context.Context, actionsBkt *bbolt.Bucket,
	al *kvdbActionLocator) (*Action, error) {

	sessBucket := actionsBkt.Bucket(al.SessionID[:])
	if sessBucket == nil {
		return nil, fmt.Errorf("session bucket for session ID "+
			"%x does not exist", al.SessionID)
	}

	var id [8]byte
	binary.BigEndian.PutUint64(id[:], al.ActionID)

	actionBytes := sessBucket.Get(id[:])

	return db.unmarshalAction(ctx, actionBytes, al.SessionID)
}

func (db *DB) unmarshalAction(ctx context.Context, actionBytes []byte,
	sessionID session.ID) (*Action, error) {

	action, err := DeserializeAction(bytes.NewReader(actionBytes), sessionID)
	if err != nil {
		return nil, err
	}

	// Now, we do a best effort to see if we should couple this action to
	// a session and/or account.
	//
	// First, check if it is linked to a session.
	macID := action.MacaroonIdentifier
	_, err = db.sessionIDIndex.GetSessionByID(ctx, macID)
	if err != nil && !errors.Is(err, session.ErrSessionNotFound) {
		return nil, err
	} else if err == nil {
		action.SessionID = fn.Some(session.ID(macID))
	}

	// Now, check if it is linked to an account.
	acct, err := db.accountsDB.GetAccountByIDPrefix(ctx, macID)
	if err != nil && !errors.Is(err, accounts.ErrAccNotFound) {
		return nil, err
	} else if err == nil {
		action.AccountID = fn.Some(acct.ID)
	}

	return action, nil
}

// SetActionState finds the action specified by the ActionLocator and sets its
// state to the given state.
//
// NOTE: This is part of the ActionDB interface.
func (db *DB) SetActionState(ctx context.Context, al ActionLocator,
	state ActionState, errorReason string) error {

	if errorReason != "" && state != ActionStateError {
		return fmt.Errorf("error reason should only be set for " +
			"ActionStateError")
	}

	locator, ok := al.(*kvdbActionLocator)
	if !ok {
		return fmt.Errorf("expected kvdbActionLocator, got %T", al)
	}

	return db.DB.Update(func(tx *bbolt.Tx) error {
		mainActionsBucket, err := getBucket(tx, actionsBucketKey)
		if err != nil {
			return err
		}

		actionsBucket := mainActionsBucket.Bucket(actionsKey)
		if actionsBucket == nil {
			return ErrNoSuchKeyFound
		}

		action, err := db.getAction(ctx, actionsBucket, locator)
		if err != nil {
			return err
		}

		action.State = state
		action.ErrorReason = errorReason

		return putAction(tx, locator, action)
	})
}

// listActionsFilterFn defines a function that can be used to determine if an
// action should be included in a set of results or not. The reversed parameter
// indicates if the actions are being traversed in reverse order or not.
// The first return boolean indicates if the action should be included or not
// and the second one indicates if the iteration should be stopped or not.
type listActionsFilterFn func(a *Action, reversed bool) (bool, bool)

// ListActions returns a list of Actions. The query IndexOffset and MaxNum
// params can be used to control the number of actions returned.
// ListActionOptions may be used to filter on specific Action values. The return
// values are the list of actions, the last index and the total count (iff
// query.CountTotal is set).
//
// NOTE: This is part of the ActionsDB interface.
func (db *DB) ListActions(ctx context.Context, query *ListActionsQuery,
	options ...ListActionOption) ([]*Action, uint64, uint64, error) {

	opts := newListActionOptions()
	for _, o := range options {
		o(opts)
	}

	filterFn := func(a *Action, reversed bool) (bool, bool) {
		timeStamp := a.AttemptedAt
		if !opts.endTime.IsZero() {
			// If actions are being considered in order and the
			// timestamp of this action exceeds the given end
			// timestamp, then there is no need to continue
			// traversing.
			if !reversed && timeStamp.After(opts.endTime) {
				return false, false
			}

			// If the actions are in reverse order and the timestamp
			// comes after the end timestamp, then the actions is
			// not included but the search can continue.
			if reversed && timeStamp.After(opts.endTime) {
				return false, true
			}
		}

		if !opts.startTime.IsZero() {
			// If actions are being considered in order and the
			// timestamp of this action comes before the given start
			// timestamp, then the action is not included but the
			// search can continue.
			if !reversed && timeStamp.Before(opts.startTime) {
				return false, true
			}

			// If the actions are in reverse order and the timestamp
			// comes before the start timestamp, then there is no
			// need to continue traversing.
			if reversed && timeStamp.Before(opts.startTime) {
				return false, false
			}
		}

		if opts.featureName != "" && a.FeatureName != opts.featureName {
			return false, true
		}

		if opts.actorName != "" && a.ActorName != opts.actorName {
			return false, true
		}

		if opts.methodName != "" && a.RPCMethod != opts.methodName {
			return false, true
		}

		if opts.state != ActionStateUnknown && a.State != opts.state {
			return false, true
		}

		return true, true
	}

	if opts.sessionID != session.EmptyID {
		return db.listSessionActions(
			ctx, opts.sessionID, filterFn, query,
		)
	}
	if opts.groupID != session.EmptyID {
		actions, err := db.listGroupActions(ctx, opts.groupID, filterFn)
		if err != nil {
			return nil, 0, 0, err
		}

		return actions, 0, uint64(len(actions)), nil
	}

	var (
		actions    []*Action
		totalCount uint64
		lastIndex  uint64
	)
	err := db.View(func(tx *bbolt.Tx) error {
		mainActionsBucket, err := getBucket(tx, actionsBucketKey)
		if err != nil {
			return err
		}

		actionsBucket := mainActionsBucket.Bucket(actionsKey)
		if actionsBucket == nil {
			return ErrNoSuchKeyFound
		}

		actionsIndexBucket := mainActionsBucket.Bucket(actionsIndex)
		if actionsIndexBucket == nil {
			return ErrNoSuchKeyFound
		}

		readAction := func(index, locatorBytes []byte) (*Action,
			error) {

			locator, err := deserializeActionLocator(
				bytes.NewReader(locatorBytes),
			)
			if err != nil {
				return nil, err
			}

			return db.getAction(ctx, actionsBucket, locator)
		}

		actions, lastIndex, totalCount, err = paginateActions(
			query, actionsIndexBucket.Cursor(), readAction,
			filterFn,
		)
		return err
	})
	if err != nil {
		return nil, 0, 0, err
	}

	return actions, lastIndex, totalCount, nil
}

// listSessionActions returns a list of the given session's Actions that pass
// the filterFn requirements.
func (db *DB) listSessionActions(ctx context.Context, sessionID session.ID,
	filterFn listActionsFilterFn, query *ListActionsQuery) ([]*Action,
	uint64, uint64, error) {

	// First, check that this session actually exists in the sessions DB.
	_, err := db.sessionIDIndex.GetGroupID(ctx, sessionID)
	if err != nil {
		return nil, 0, 0, err
	}

	var (
		actions    []*Action
		totalCount uint64
		lastIndex  uint64
	)
	err = db.View(func(tx *bbolt.Tx) error {
		mainActionsBucket, err := getBucket(tx, actionsBucketKey)
		if err != nil {
			return err
		}

		actionsBucket := mainActionsBucket.Bucket(actionsKey)
		if actionsBucket == nil {
			return ErrNoSuchKeyFound
		}

		sessionsBucket := actionsBucket.Bucket(sessionID[:])
		if sessionsBucket == nil {
			return nil
		}

		readAction := func(_, v []byte) (*Action, error) {
			return db.unmarshalAction(ctx, v, sessionID)
		}

		actions, lastIndex, totalCount, err = paginateActions(
			query, sessionsBucket.Cursor(), readAction, filterFn,
		)

		return err
	})
	if err != nil {
		return nil, 0, 0, err
	}

	return actions, lastIndex, totalCount, nil
}

// listGroupActions returns a list of the given session group's Actions that
// pass the filterFn requirements.
//
// TODO: update to allow for pagination.
func (db *DB) listGroupActions(ctx context.Context, groupID session.ID,
	filterFn listActionsFilterFn) ([]*Action, error) {

	if filterFn == nil {
		filterFn = func(a *Action, reversed bool) (bool, bool) {
			return true, true
		}
	}

	sessionIDs, err := db.sessionIDIndex.GetSessionIDs(ctx, groupID)
	if err != nil {
		return nil, err
	}

	var (
		actions []*Action
		errDone = errors.New("done iterating")
	)
	err = db.View(func(tx *bbolt.Tx) error {
		mainActionsBucket, err := getBucket(tx, actionsBucketKey)
		if err != nil {
			return err
		}

		actionsBucket := mainActionsBucket.Bucket(actionsKey)
		if actionsBucket == nil {
			return ErrNoSuchKeyFound
		}

		// Iterate over each session ID in this group.
		for _, sessionID := range sessionIDs {
			sessionsBucket := actionsBucket.Bucket(sessionID[:])
			if sessionsBucket == nil {
				return nil
			}

			err = sessionsBucket.ForEach(func(_, v []byte) error {
				action, err := db.unmarshalAction(
					ctx, v, sessionID,
				)
				if err != nil {
					return err
				}

				include, cont := filterFn(action, false)
				if include {
					actions = append(actions, action)
				}

				if !cont {
					return errDone
				}

				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil && !errors.Is(err, errDone) {
		return nil, err
	}

	return actions, nil
}

// GetActionsReadDB is a method on DB that constructs an ActionsReadDB.
//
// NOTE: This is part of the ActionDB interface.
func (db *DB) GetActionsReadDB(groupID session.ID,
	featureName string) ActionsReadDB {

	return &allActionsReadDB{
		db:          db,
		groupID:     groupID,
		featureName: featureName,
	}
}

// SerializeAction binary serializes the given action to the writer using the
// tlv format.
func SerializeAction(w io.Writer, action *Action) error {
	if action == nil {
		return fmt.Errorf("action cannot be nil")
	}

	var (
		actor       = []byte(action.ActorName)
		feature     = []byte(action.FeatureName)
		trigger     = []byte(action.Trigger)
		intent      = []byte(action.Intent)
		data        = []byte(action.StructuredJsonData)
		rpcMethod   = []byte(action.RPCMethod)
		params      = action.RPCParamsJson
		attemptedAt = uint64(action.AttemptedAt.Unix())
		state       = uint8(action.State)
		errorReason = []byte(action.ErrorReason)
	)

	tlvRecords := []tlv.Record{
		tlv.MakePrimitiveRecord(typeActorName, &actor),
		tlv.MakePrimitiveRecord(typeFeature, &feature),
		tlv.MakePrimitiveRecord(typeTrigger, &trigger),
		tlv.MakePrimitiveRecord(typeIntent, &intent),
		tlv.MakePrimitiveRecord(typeStructuredJsonData, &data),
		tlv.MakePrimitiveRecord(typeRPCMethod, &rpcMethod),
		tlv.MakePrimitiveRecord(typeRPCParamsJson, &params),
		tlv.MakePrimitiveRecord(typeAttemptedAt, &attemptedAt),
		tlv.MakePrimitiveRecord(typeState, &state),
		tlv.MakePrimitiveRecord(typeErrorReason, &errorReason),
	}

	tlvStream, err := tlv.NewStream(tlvRecords...)
	if err != nil {
		return err
	}

	return tlvStream.Encode(w)
}

// DeserializeAction deserializes an action from the given reader, expecting
// the data to be encoded in the tlv format.
func DeserializeAction(r io.Reader, sessionID session.ID) (*Action, error) {
	var (
		action                = Action{}
		actor, featureName    []byte
		trigger, intent, data []byte
		rpcMethod, params     []byte
		attemptedAt           uint64
		state                 uint8
		errorReason           []byte
	)
	tlvStream, err := tlv.NewStream(
		tlv.MakePrimitiveRecord(typeActorName, &actor),
		tlv.MakePrimitiveRecord(typeFeature, &featureName),
		tlv.MakePrimitiveRecord(typeTrigger, &trigger),
		tlv.MakePrimitiveRecord(typeIntent, &intent),
		tlv.MakePrimitiveRecord(typeStructuredJsonData, &data),
		tlv.MakePrimitiveRecord(typeRPCMethod, &rpcMethod),
		tlv.MakePrimitiveRecord(typeRPCParamsJson, &params),
		tlv.MakePrimitiveRecord(typeAttemptedAt, &attemptedAt),
		tlv.MakePrimitiveRecord(typeState, &state),
		tlv.MakePrimitiveRecord(typeErrorReason, &errorReason),
	)
	if err != nil {
		return nil, err
	}

	_, err = tlvStream.DecodeWithParsedTypes(r)
	if err != nil {
		return nil, err
	}

	action.MacaroonIdentifier = sessionID
	action.ActorName = string(actor)
	action.FeatureName = string(featureName)
	action.Trigger = string(trigger)
	action.Intent = string(intent)
	action.StructuredJsonData = string(data)
	action.RPCMethod = string(rpcMethod)
	action.RPCParamsJson = params
	action.AttemptedAt = time.Unix(int64(attemptedAt), 0)
	action.State = ActionState(state)
	action.ErrorReason = string(errorReason)

	return &action, nil
}

// serializeActionLocator binary serializes the given kvdbActionLocator to the
// writer using the tlv format.
func serializeActionLocator(w io.Writer, al *kvdbActionLocator) error {
	if al == nil {
		return fmt.Errorf("action locator cannot be nil")
	}

	var (
		sessionID = al.SessionID[:]
		actionID  = al.ActionID
	)

	tlvRecords := []tlv.Record{
		tlv.MakePrimitiveRecord(typeLocatorSessionID, &sessionID),
		tlv.MakePrimitiveRecord(typeLocatorActionID, &actionID),
	}

	tlvStream, err := tlv.NewStream(tlvRecords...)
	if err != nil {
		return err
	}

	return tlvStream.Encode(w)
}

// deserializeActionLocator deserializes an kvdbActionLocator from the given
// reader, expecting the data to be encoded in the tlv format.
func deserializeActionLocator(r io.Reader) (*kvdbActionLocator, error) {
	var (
		sessionID []byte
		actionID  uint64
	)
	tlvStream, err := tlv.NewStream(
		tlv.MakePrimitiveRecord(typeLocatorSessionID, &sessionID),
		tlv.MakePrimitiveRecord(typeLocatorActionID, &actionID),
	)
	if err != nil {
		return nil, err
	}

	_, err = tlvStream.DecodeWithParsedTypes(r)
	if err != nil {
		return nil, err
	}

	id, err := session.IDFromBytes(sessionID)
	if err != nil {
		return nil, err
	}

	return &kvdbActionLocator{
		SessionID: id,
		ActionID:  actionID,
	}, nil
}

// kvdbActionLocator helps us find an action in a KVDB database.
type kvdbActionLocator struct {
	SessionID session.ID
	ActionID  uint64
}

// A compile-time check to ensure kvdbActionLocator implements the ActionLocator
// interface.
var _ ActionLocator = (*kvdbActionLocator)(nil)

func (al *kvdbActionLocator) isActionLocator() {}

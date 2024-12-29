package session

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightninglabs/lightning-terminal/db"
	"github.com/lightninglabs/lightning-terminal/db/sqlc"
	"gopkg.in/macaroon-bakery.v2/bakery"
	"gopkg.in/macaroon.v2"
)

// SQLQueries is a subset of the sqlc.Queries interface that can be used to
// interact with session related tables.
type SQLQueries interface {
	GetLegacyIDBySessionID(ctx context.Context, id int64) ([]byte, error)
	GetSessionByID(ctx context.Context, id int64) (sqlc.Session, error)
	GetSessionsInGroup(ctx context.Context, groupID sql.NullInt64) ([]sqlc.Session, error)
	GetSessionLegacyIDsInGroup(ctx context.Context, groupID sql.NullInt64) ([][]byte, error)
	GetSessionByLegacyID(ctx context.Context, legacyID []byte) (sqlc.Session, error)
	GetSessionByLocalPublicKey(ctx context.Context, localPublicKey []byte) (sqlc.Session, error)
	GetSessionFeatureConfigs(ctx context.Context, sessionID int64) ([]sqlc.FeatureConfig, error)
	GetSessionMacaroonCaveats(ctx context.Context, sessionID int64) ([]sqlc.MacaroonCaveat, error)
	GetSessionIDByLegacyID(ctx context.Context, legacyID []byte) (int64, error)
	GetSessionMacaroonPermissions(ctx context.Context, sessionID int64) ([]sqlc.MacaroonPermission, error)
	GetSessionPrivacyFlags(ctx context.Context, sessionID int64) ([]sqlc.PrivacyFlag, error)
	InsertFeatureConfig(ctx context.Context, arg sqlc.InsertFeatureConfigParams) error
	InsertMacaroonCaveat(ctx context.Context, arg sqlc.InsertMacaroonCaveatParams) error
	InsertMacaroonPermission(ctx context.Context, arg sqlc.InsertMacaroonPermissionParams) error
	InsertPrivacyFlag(ctx context.Context, arg sqlc.InsertPrivacyFlagParams) error
	InsertSession(ctx context.Context, arg sqlc.InsertSessionParams) (int64, error)
	ListSessions(ctx context.Context) ([]sqlc.Session, error)
	ListSessionsByType(ctx context.Context, sessionType int16) ([]sqlc.Session, error)
	SetRemotePublicKeyByLocalPublicKey(ctx context.Context, arg sqlc.SetRemotePublicKeyByLocalPublicKeyParams) error
	SetSessionGroupID(ctx context.Context, arg sqlc.SetSessionGroupIDParams) error
	UpdateSessionState(ctx context.Context, arg sqlc.UpdateSessionStateParams) error
}

var _ Store = (*SQLStore)(nil)

// BatchedSQLQueries is a version of the SQLQueries that's capable of batched
// database operations.
type BatchedSQLQueries interface {
	SQLQueries

	db.BatchedTx[SQLQueries]
}

// SQLStore represents a storage backend.
type SQLStore struct {
	db BatchedSQLQueries
}

// NewSQLStore creates a new SQLStore instance given an open BatchedSQLQueries
// storage backend.
func NewSQLStore(db BatchedSQLQueries) *SQLStore {
	return &SQLStore{
		db: db,
	}
}

// CreateSession adds a new session to the store. If a session with the same
// local public key already exists an error is returned. This can only be called
// with a Session with an ID that the Store has reserved.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) CreateSession(ctx context.Context, sess *Session) error {
	var writeTxOpts db.QueriesTxOptions

	err := s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		localKey := sess.LocalPublicKey.SerializeCompressed()

		id, err := db.InsertSession(ctx, sqlc.InsertSessionParams{
			LegacyID:        sess.ID[:],
			Label:           sess.Label,
			State:           int16(sess.State),
			Type:            int16(sess.Type),
			Expiry:          sess.Expiry,
			CreatedAt:       sess.CreatedAt,
			ServerAddress:   sess.ServerAddr,
			DevServer:       sess.DevServer,
			MacaroonRootKey: int64(sess.MacaroonRootKey),
			PairingSecret:   sess.PairingSecret[:],
			LocalPrivateKey: sess.LocalPrivateKey.Serialize(),
			LocalPublicKey:  localKey,
			Privacy:         sess.WithPrivacyMapper,
		})
		if err != nil {
			return fmt.Errorf("unable to insert session: %w", err)
		}

		// Check that the linked session is known.
		groupID, err := db.GetSessionIDByLegacyID(ctx, sess.GroupID[:])
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUnknownGroup
		} else if err != nil {
			return fmt.Errorf("unable to fetch group(%x): %w",
				sess.GroupID[:], err)
		}

		// Ensure that all other sessions in this group are no longer
		// active.
		linkedSessions, err := db.GetSessionsInGroup(ctx, sql.NullInt64{
			Int64: groupID,
			Valid: true,
		})
		if err != nil {
			return fmt.Errorf("unable to fetch group(%x): %w",
				sess.GroupID[:], err)
		}

		// Make sure that all linked sessions (sessions in the same
		// group) are no longer active.
		for _, linkedSession := range linkedSessions {
			// Skip the new session that we are adding.
			if linkedSession.ID == id {
				continue
			}

			// Any other session should not be active.
			if State(linkedSession.State) == StateCreated ||
				State(linkedSession.State) == StateInUse {

				return fmt.Errorf("linked session(%x) is "+
					"still active: %w",
					linkedSession.LegacyID[:],
					ErrSessionsInGroupStillActive)
			}
		}

		err = db.SetSessionGroupID(ctx, sqlc.SetSessionGroupIDParams{
			ID: id,
			GroupID: sql.NullInt64{
				Int64: groupID,
				Valid: true,
			},
		})
		if err != nil {
			return fmt.Errorf("unable to set group ID: %w", err)
		}

		// Write mac perms and caveats.
		if sess.MacaroonRecipe != nil {
			for _, perm := range sess.MacaroonRecipe.Permissions {
				err := db.InsertMacaroonPermission(
					ctx, sqlc.InsertMacaroonPermissionParams{
						SessionID: id,
						Entity:    perm.Entity,
						Action:    perm.Action,
					},
				)
				if err != nil {
					return fmt.Errorf("unable to insert "+
						"mac perm: %w", err)
				}
			}

			for _, caveat := range sess.MacaroonRecipe.Caveats {
				err := db.InsertMacaroonCaveat(
					ctx, sqlc.InsertMacaroonCaveatParams{
						SessionID: id,
						ID:        caveat.Id,
						VerificationID: caveat.
							VerificationId,
						Location: sql.NullString{
							String: caveat.Location,
							Valid: caveat.
								Location != "",
						},
					},
				)
				if err != nil {
					return fmt.Errorf("unable to insert "+
						"mac caveat: %v", err)
				}
			}
		}

		// Write feature configs.
		if sess.FeatureConfig != nil {
			for featureName, config := range *sess.FeatureConfig {
				err := db.InsertFeatureConfig(
					ctx, sqlc.InsertFeatureConfigParams{
						SessionID:   id,
						FeatureName: featureName,
						Config:      config,
					},
				)
				if err != nil {
					return fmt.Errorf("unable to insert "+
						"feature config: %w", err)
				}
			}
		}

		// Write privacy flags.
		for _, flag := range sess.PrivacyFlags {
			err := db.InsertPrivacyFlag(
				ctx, sqlc.InsertPrivacyFlagParams{
					SessionID: id,
					Flag:      int32(flag),
				},
			)
			if err != nil {
				return fmt.Errorf("unable to insert privacy "+
					"flag: %w", err)
			}
		}

		return nil
	}, func() {})
	if err != nil {
		mappedSQLErr := db.MapSQLError(err)
		var uniqueConstraintErr *db.ErrSQLUniqueConstraintViolation
		if errors.As(mappedSQLErr, &uniqueConstraintErr) {
			// Add context to unique constraint errors.
			return ErrSessionExists
		}

		return fmt.Errorf("unable to add session(%x): %w", sess.ID, err)
	}

	return nil
}

// GetSession fetches the session with the given local pub key.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) GetSession(ctx context.Context, key *btcec.PublicKey) (
	*Session, error) {

	var (
		readTxOpts = db.NewQueryReadTx()
		sess       *Session
	)
	err := s.db.ExecTx(ctx, &readTxOpts, func(db SQLQueries) error {
		dbSess, err := db.GetSessionByLocalPublicKey(
			ctx, key.SerializeCompressed(),
		)
		if err != nil {
			return fmt.Errorf("unable to get session: %w", err)
		}

		sess, err = unmarshalSession(ctx, s.db, dbSess)
		if err != nil {
			return fmt.Errorf("unable to unmarshal session: %w",
				err)
		}

		return nil
	}, func() {})
	if err != nil {
		return nil, err
	}

	return sess, nil
}

// ListSessions returns all sessions currently known to the store. The filterFn
// can be used to filter the sessions returned.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) ListSessions(ctx context.Context,
	filterFn func(s *Session) bool) ([]*Session, error) {

	var (
		readTxOpts = db.NewQueryReadTx()
		sessions   []*Session
	)
	err := s.db.ExecTx(ctx, &readTxOpts, func(db SQLQueries) error {
		dbSessions, err := db.ListSessions(ctx)
		if err != nil {
			return fmt.Errorf("could not list sessions: %w", err)
		}

		for _, dbSess := range dbSessions {
			sess, err := unmarshalSession(ctx, db, dbSess)
			if err != nil {
				return fmt.Errorf("could not unmarshal "+
					"session: %w", err)
			}

			if filterFn != nil && !filterFn(sess) {
				return nil
			}

			sessions = append(sessions, sess)
		}

		return nil
	}, func() {})

	return sessions, err
}

// RevokeSession updates the state of the session with the given local public
// key to be revoked.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) RevokeSession(ctx context.Context,
	key *btcec.PublicKey) error {

	var writeTxOpts db.QueriesTxOptions
	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		dbSess, err := s.db.GetSessionByLocalPublicKey(
			ctx, key.SerializeCompressed(),
		)
		if err != nil {
			return fmt.Errorf("unable to get session: %w", err)
		}

		return s.db.UpdateSessionState(
			ctx, sqlc.UpdateSessionStateParams{
				ID:    dbSess.ID,
				State: int16(StateRevoked),
			},
		)
	}, func() {})
}

// UpdateSessionRemotePubKey can be used to add the given remote pub key to the
// session with the given local pub key.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) UpdateSessionRemotePubKey(ctx context.Context, localPubKey,
	remotePubKey *btcec.PublicKey) error {

	var (
		writeTxOpts db.QueriesTxOptions
		remoteKey   = remotePubKey.SerializeCompressed()
		localKey    = localPubKey.SerializeCompressed()
	)
	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		return db.SetRemotePublicKeyByLocalPublicKey(
			ctx, sqlc.SetRemotePublicKeyByLocalPublicKeyParams{
				RemotePublicKey: remoteKey,
				LocalPublicKey:  localKey,
			},
		)
	}, func() {})
}

// GetUnusedIDAndKeyPair can be used to generate a new, unused, local private
// key and session ID pair. Care must be taken to ensure that no other thread
// calls this before the returned ID and key pair from this method are either
// used or discarded.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) GetUnusedIDAndKeyPair(ctx context.Context) (ID,
	*btcec.PrivateKey, error) {

	var (
		readTxOpts = db.NewQueryReadTx()
		id         ID
		privKey    *btcec.PrivateKey
		err        error
	)
	err = s.db.ExecTx(ctx, &readTxOpts, func(db SQLQueries) error {
		// Spin until we find a key with an ID that does not collide
		// with any of our existing IDs.
		for {
			// Generate a new private key and ID pair.
			privKey, id, err = NewSessionPrivKeyAndID()
			if err != nil {
				return err
			}

			// Check that no such legacy ID exits.
			_, err = db.GetSessionByLegacyID(ctx, id[:])
			if !errors.Is(err, sql.ErrNoRows) {
				continue
			}

			break
		}

		return nil
	}, func() {})

	return id, privKey, err
}

// GetSessionByID returns the session with the given legacy ID.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) GetSessionByID(ctx context.Context, legacyID ID) (*Session,
	error) {

	var (
		readTxOpts = db.NewQueryReadTx()
		sess       *Session
	)
	err := s.db.ExecTx(ctx, &readTxOpts, func(db SQLQueries) error {
		dbSess, err := db.GetSessionByLegacyID(ctx, legacyID[:])
		if errors.Is(err, sql.ErrNoRows) {
			return ErrSessionNotFound
		} else if err != nil {
			return fmt.Errorf("unable to get session: %w", err)
		}

		sess, err = unmarshalSession(ctx, s.db, dbSess)
		if err != nil {
			return fmt.Errorf("unable to unmarshal session: %w",
				err)
		}

		return nil
	}, func() {})
	if err != nil {
		return nil, err
	}

	return sess, nil
}

// CheckSessionGroupPredicate iterates over all the sessions in a group and
// checks if each one passes the given predicate function. True is returned if
// each session passes.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) CheckSessionGroupPredicate(ctx context.Context,
	legacyGroupID ID, fn func(s *Session) bool) (bool, error) {

	var (
		readTxOpts = db.NewQueryReadTx()
		passes     bool
	)
	err := s.db.ExecTx(ctx, &readTxOpts, func(db SQLQueries) error {
		groupID, err := db.GetSessionIDByLegacyID(ctx, legacyGroupID[:])
		if err != nil {
			return err
		}

		dbSessions, err := db.GetSessionsInGroup(ctx, sql.NullInt64{
			Int64: groupID,
			Valid: true,
		})
		if err != nil {
			return err
		}

		for _, dbSess := range dbSessions {
			sess, err := unmarshalSession(ctx, db, dbSess)
			if err != nil {
				return err
			}

			if !fn(sess) {
				return nil
			}
		}

		passes = true

		return nil
	}, func() {})

	return passes, err
}

// GetGroupID will return the legacy group ID for the given legacy session ID.
//
// NOTE: This is part of the IDToGroupIndex interface.
func (s *SQLStore) GetGroupID(ctx context.Context, sessionID ID) (ID, error) {
	var (
		readTxOpts    = db.NewQueryReadTx()
		legacyGroupID ID
	)
	err := s.db.ExecTx(ctx, &readTxOpts, func(db SQLQueries) error {
		// Get the session using the legacy ID.
		sess, err := db.GetSessionByLegacyID(ctx, sessionID[:])
		if err != nil {
			return err
		}

		if !sess.GroupID.Valid {
			return fmt.Errorf("session does not have a group ID")
		}

		// Get the legacy group ID using the session group ID.
		legacyGroupIDB, err := db.GetLegacyIDBySessionID(
			ctx, sess.GroupID.Int64,
		)
		if err != nil {
			return err
		}

		legacyGroupID, err = IDFromBytes(legacyGroupIDB)

		return err
	}, func() {})
	if err != nil {
		return ID{}, err
	}

	return legacyGroupID, nil
}

// GetSessionIDs will return the set of legacy session IDs that are in the
// group with the given legacy ID.
//
// NOTE: This is part of the IDToGroupIndex interface.
func (s *SQLStore) GetSessionIDs(ctx context.Context, legacyGroupID ID) ([]ID,
	error) {

	var (
		readTxOpts = db.NewQueryReadTx()
		sessionIDs []ID
	)
	err := s.db.ExecTx(ctx, &readTxOpts, func(db SQLQueries) error {
		groupID, err := db.GetSessionIDByLegacyID(ctx, legacyGroupID[:])
		if err != nil {
			return fmt.Errorf("unable to get session ID: %v", err)
		}

		sessIDs, err := db.GetSessionLegacyIDsInGroup(
			ctx, sql.NullInt64{
				Int64: groupID,
				Valid: true,
			},
		)
		if err != nil {
			return fmt.Errorf("unable to get session IDs: %v", err)
		}

		sessionIDs = make([]ID, len(sessIDs))
		for i, sessID := range sessIDs {
			id, err := IDFromBytes(sessID)
			if err != nil {
				return err
			}

			sessionIDs[i] = id
		}

		return nil
	}, func() {})
	if err != nil {
		return nil, err
	}

	return sessionIDs, nil
}

func unmarshalSession(ctx context.Context, db SQLQueries,
	dbSess sqlc.Session) (*Session, error) {

	var legacyGroupID ID
	if dbSess.GroupID.Valid {
		groupID, err := db.GetLegacyIDBySessionID(
			ctx, dbSess.GroupID.Int64,
		)
		if err != nil {
			return nil, fmt.Errorf("unable to get legacy group "+
				"ID: %v", err)
		}

		legacyGroupID, err = IDFromBytes(groupID)
		if err != nil {
			return nil, fmt.Errorf("unable to get legacy ID: %v",
				err)
		}
	}

	legacyID, err := IDFromBytes(dbSess.LegacyID)
	if err != nil {
		return nil, fmt.Errorf("unable to get legacy ID: %v", err)
	}

	var revokedAt time.Time
	if dbSess.RevokedAt.Valid {
		revokedAt = dbSess.RevokedAt.Time
	}

	localPriv, localPub := btcec.PrivKeyFromBytes(dbSess.LocalPrivateKey)

	var remotePub *btcec.PublicKey
	if len(dbSess.RemotePublicKey) != 0 {
		remotePub, err = btcec.ParsePubKey(dbSess.RemotePublicKey)
		if err != nil {
			return nil, fmt.Errorf("unable to parse remote "+
				"public key: %v", err)
		}
	}

	// Get the macaroon permissions if they exist.
	perms, err := db.GetSessionMacaroonPermissions(ctx, dbSess.ID)
	if err != nil {
		return nil, fmt.Errorf("unable to get macaroon "+
			"permissions: %v", err)
	}

	// Get the macaroon caveats if they exist.
	caveats, err := db.GetSessionMacaroonCaveats(ctx, dbSess.ID)
	if err != nil {
		return nil, fmt.Errorf("unable to get macaroon "+
			"caveats: %v", err)
	}

	var macRecipe *MacaroonRecipe
	if perms != nil || caveats != nil {
		macRecipe = &MacaroonRecipe{
			Permissions: unmarshalMacPerms(perms),
			Caveats:     unmarshalMacCaveats(caveats),
		}
	}

	// Get the feature configs if they exist.
	featureConfigs, err := db.GetSessionFeatureConfigs(ctx, dbSess.ID)
	if err != nil {
		return nil, fmt.Errorf("unable to get feature configs: %v", err)
	}

	var featureCfgs *FeaturesConfig
	if featureConfigs != nil {
		featureCfgs = unmarshalFeatureConfigs(featureConfigs)
	}

	// Get the privacy flags if they exist.
	privacyFlags, err := db.GetSessionPrivacyFlags(ctx, dbSess.ID)
	if err != nil {
		return nil, fmt.Errorf("unable to get privacy flags: %v", err)
	}

	var privFlags PrivacyFlags
	if privacyFlags != nil {
		privFlags = unmarshalPrivacyFlags(privacyFlags)
	}

	var pairingSecret [14]byte
	copy(pairingSecret[:], dbSess.PairingSecret)

	return &Session{
		ID:                legacyID,
		Label:             dbSess.Label,
		State:             State(dbSess.State),
		Type:              Type(dbSess.Type),
		Expiry:            dbSess.Expiry,
		CreatedAt:         dbSess.CreatedAt,
		RevokedAt:         revokedAt,
		ServerAddr:        dbSess.ServerAddress,
		DevServer:         dbSess.DevServer,
		MacaroonRootKey:   uint64(dbSess.MacaroonRootKey),
		PairingSecret:     pairingSecret,
		LocalPrivateKey:   localPriv,
		LocalPublicKey:    localPub,
		RemotePublicKey:   remotePub,
		WithPrivacyMapper: dbSess.Privacy,
		GroupID:           legacyGroupID,
		PrivacyFlags:      privFlags,
		MacaroonRecipe:    macRecipe,
		FeatureConfig:     featureCfgs,
	}, nil
}

func unmarshalMacPerms(dbPerms []sqlc.MacaroonPermission) []bakery.Op {
	ops := make([]bakery.Op, 0, len(dbPerms))
	for i, dbPerm := range dbPerms {
		ops[i] = bakery.Op{
			Entity: dbPerm.Entity,
			Action: dbPerm.Action,
		}
	}

	return ops
}

func unmarshalMacCaveats(dbCaveats []sqlc.MacaroonCaveat) []macaroon.Caveat {
	caveats := make([]macaroon.Caveat, 0, len(dbCaveats))
	for i, dbCaveat := range dbCaveats {
		caveats[i] = macaroon.Caveat{
			Id:             dbCaveat.ID,
			VerificationId: dbCaveat.VerificationID,
			Location:       dbCaveat.Location.String,
		}
	}

	return caveats
}

func unmarshalFeatureConfigs(dbConfigs []sqlc.FeatureConfig) *FeaturesConfig {
	configs := make(FeaturesConfig, len(dbConfigs))
	for _, dbConfig := range dbConfigs {
		configs[dbConfig.FeatureName] = dbConfig.Config
	}

	return &configs
}

func unmarshalPrivacyFlags(dbFlags []sqlc.PrivacyFlag) PrivacyFlags {
	flags := make(PrivacyFlags, len(dbFlags))
	for i, dbFlag := range dbFlags {
		flags[i] = PrivacyFlag(dbFlag.Flag)
	}

	return flags
}

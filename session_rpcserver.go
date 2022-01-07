package terminal

import (
	"context"
	"encoding/hex"
	"fmt"
	"google.golang.org/grpc"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/lightninglabs/lightning-node-connect/mailbox"
	"github.com/lightninglabs/lightning-terminal/litrpc"
	"github.com/lightninglabs/lightning-terminal/session"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/keychain"
)

var (
	// TODO(elle): Temp values. change these before landing pr.
	nBytes, _ = hex.DecodeString(
		"0254a58cd0f31c008fd0bc9b2dd5ba586144933829f6da33ac4130b555fb5ea32c",
	)
	N, _ = btcec.ParsePubKey(nBytes, btcec.S256())

	keyLocator = &keychain.KeyLocator{
		Family: 32,
		Index:  10,
	}
)

// sessionRpcServer is the gRPC server for the Session RPC interface.
type sessionRpcServer struct {
	litrpc.UnimplementedSessionsServer

	basicAuth string

	cfg             *sessionRpcServerConfig
	db              *session.DB
	sessionServer   *session.Server
	macaroonService *lndclient.MacaroonService
}

// sessionRpcServerConfig holds the values used to configure the
// sessionRpcServer.
type sessionRpcServerConfig struct {
	basicAuth    string
	dbDir        string
	grpcOptions  []grpc.ServerOption
	macaroonPath string
}

// newSessionRPCServer creates a new sessionRpcServer using the passed config.
func newSessionRPCServer(cfg *sessionRpcServerConfig) (*sessionRpcServer,
	error) {

	// Create an instance of the local Terminal Connect session store DB.
	db, err := session.NewDB(cfg.dbDir, session.DBFilename)
	if err != nil {
		return nil, fmt.Errorf("error creating session DB: %v", err)
	}

	// Create the gRPC server that handles adding/removing sessions and the
	// actual mailbox server that spins up the Terminal Connect server
	// interface.
	server := session.NewServer(
		func(opts ...grpc.ServerOption) *grpc.Server {
			allOpts := append(cfg.grpcOptions, opts...)
			return grpc.NewServer(allOpts...)
		},
	)

	return &sessionRpcServer{
		cfg:           cfg,
		basicAuth:     cfg.basicAuth,
		db:            db,
		sessionServer: server,
	}, nil
}

// start all the components necessary for the sessionRpcServer to start serving
// requests. This includes starting the macaroon service and resuming all
// non-revoked sessions.
func (s *sessionRpcServer) start(stateless bool,
	lndClient *lndclient.LndServices) error {

	var err error
	s.macaroonService, err = lndclient.NewMacaroonService(
		&lndclient.MacaroonServiceConfig{
			DBPath:           s.cfg.dbDir,
			MacaroonLocation: "litd",
			StatelessInit:    stateless,
			RequiredPerms:    litPermissions,
			LndClient:        lndClient,
			EphemeralKey:     N,
			KeyLocator:       keyLocator,
			MacaroonPath:     s.cfg.macaroonPath,
		},
	)
	if err != nil {
		log.Errorf("Could not create a new macaroon service: %v", err)
		return err
	}

	if err := s.macaroonService.Start(); err != nil {
		log.Errorf("Could not start macaroon service: %v", err)
		return err
	}

	// Start up all previously created sessions.
	sessions, err := s.db.ListSessions()
	if err != nil {
		return fmt.Errorf("error listing sessions: %v", err)
	}
	for _, sess := range sessions {
		if err := s.resumeSession(sess); err != nil {
			return fmt.Errorf("error resuming sesion: %v", err)
		}
	}

	return nil
}

// stop cleans up all resources managed by sessionRpcServer.
func (s *sessionRpcServer) stop() error {
	var returnErr error
	if err := s.db.Close(); err != nil {
		log.Errorf("Error closing session DB: %v", err)
		returnErr = err
	}
	s.sessionServer.Stop()

	if err := s.macaroonService.Stop(); err != nil {
		log.Errorf("Error stopping macaroon service: %v", err)
		returnErr = err
	}

	return returnErr
}

// AddSession adds and starts a new Terminal Connect session.
func (s *sessionRpcServer) AddSession(_ context.Context,
	req *litrpc.AddSessionRequest) (*litrpc.AddSessionResponse, error) {

	var (
		typ    session.Type
		expiry time.Time
	)
	switch req.SessionType {
	case litrpc.SessionType_TYPE_UI_PASSWORD:
		typ = session.TypeUIPassword

	default:
		return nil, fmt.Errorf("invalid session type, only UI " +
			"password supported in LiT")
	}

	expiry = time.Unix(int64(req.ExpiryTimestampSeconds), 0)
	if time.Now().After(expiry) {
		return nil, fmt.Errorf("expiry must be in the future")
	}

	sess, err := session.NewSession(
		req.Label, typ, expiry, req.MailboxServerAddr, req.DevServer,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating new session: %v", err)
	}

	if err := s.db.StoreSession(sess); err != nil {
		return nil, fmt.Errorf("error storing session: %v", err)
	}

	if err := s.resumeSession(sess); err != nil {
		return nil, fmt.Errorf("error starting session: %v", err)
	}

	rpcSession, err := marshalRPCSession(sess)
	if err != nil {
		return nil, fmt.Errorf("error marshaling session: %v", err)
	}

	return &litrpc.AddSessionResponse{
		Session: rpcSession,
	}, nil
}

// resumeSession tries to start an existing session if it is not expired, not
// revoked and a LiT session.
func (s *sessionRpcServer) resumeSession(sess *session.Session) error {
	// We only start non-revoked, non-expired LiT sessions. Everything else
	// we just skip.
	if sess.State != session.StateInUse &&
		sess.State != session.StateCreated {

		log.Debugf("Not resuming session %x with state %d",
			sess.LocalPublicKey.SerializeCompressed(), sess.State)
		return nil
	}
	if sess.Type != session.TypeUIPassword {
		log.Debugf("Not resuming session %x with type %d",
			sess.LocalPublicKey.SerializeCompressed(), sess.Type)
		return nil
	}
	if sess.Expiry.Before(time.Now()) {
		log.Debugf("Not resuming session %x with expiry %s",
			sess.LocalPublicKey.SerializeCompressed(), sess.Expiry)
		return nil
	}

	authData := []byte("Authorization: Basic " + s.basicAuth)
	return s.sessionServer.StartSession(sess, authData)
}

// ListSessions returns all sessions known to the session store.
func (s *sessionRpcServer) ListSessions(_ context.Context,
	_ *litrpc.ListSessionsRequest) (*litrpc.ListSessionsResponse, error) {

	sessions, err := s.db.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("error fetching sessions: %v", err)
	}

	response := &litrpc.ListSessionsResponse{
		Sessions: make([]*litrpc.Session, len(sessions)),
	}
	for idx, sess := range sessions {
		response.Sessions[idx], err = marshalRPCSession(sess)
		if err != nil {
			return nil, fmt.Errorf("error marshaling session: %v",
				err)
		}
	}

	return response, nil
}

// RevokeSession revokes a single session and also stops it if it is currently
// active.
func (s *sessionRpcServer) RevokeSession(_ context.Context,
	req *litrpc.RevokeSessionRequest) (*litrpc.RevokeSessionResponse, error) {

	pubKey, err := btcec.ParsePubKey(req.LocalPublicKey, btcec.S256())
	if err != nil {
		return nil, fmt.Errorf("error parsing public key: %v", err)
	}

	if err := s.db.RevokeSession(pubKey); err != nil {
		return nil, fmt.Errorf("error revoking session: %v", err)
	}

	// If the session expired already it might not be running anymore. So we
	// only log possible errors here.
	if err := s.sessionServer.StopSession(pubKey); err != nil {
		log.Debugf("Error stopping session: %v", err)
	}

	return &litrpc.RevokeSessionResponse{}, nil
}

// marshalRPCSession converts a session into its RPC counterpart.
func marshalRPCSession(sess *session.Session) (*litrpc.Session, error) {
	rpcState, err := marshalRPCState(sess.State)
	if err != nil {
		return nil, err
	}

	rpcType, err := marshalRPCType(sess.Type)
	if err != nil {
		return nil, err
	}

	var remotePubKey []byte
	if sess.RemotePublicKey != nil {
		remotePubKey = sess.RemotePublicKey.SerializeCompressed()
	}

	mnemonic, err := mailbox.PasswordEntropyToMnemonic(sess.PairingSecret)
	if err != nil {
		return nil, err
	}

	return &litrpc.Session{
		Label:                  sess.Label,
		SessionState:           rpcState,
		SessionType:            rpcType,
		ExpiryTimestampSeconds: uint64(sess.Expiry.Unix()),
		MailboxServerAddr:      sess.ServerAddr,
		DevServer:              sess.DevServer,
		PairingSecret:          sess.PairingSecret[:],
		PairingSecretMnemonic:  strings.Join(mnemonic[:], " "),
		LocalPublicKey:         sess.LocalPublicKey.SerializeCompressed(),
		RemotePublicKey:        remotePubKey,
	}, nil
}

// marshalRPCState converts a session state to its RPC counterpart.
func marshalRPCState(state session.State) (litrpc.SessionState, error) {
	switch state {
	case session.StateCreated:
		return litrpc.SessionState_STATE_CREATED, nil

	case session.StateInUse:
		return litrpc.SessionState_STATE_IN_USE, nil

	case session.StateRevoked:
		return litrpc.SessionState_STATE_REVOKED, nil

	case session.StateExpired:
		return litrpc.SessionState_STATE_EXPIRED, nil

	default:
		return 0, fmt.Errorf("unknown state <%d>", state)
	}
}

// marshalRPCType converts a session type to its RPC counterpart.
func marshalRPCType(typ session.Type) (litrpc.SessionType, error) {
	switch typ {
	case session.TypeMacaroonReadonly:
		return litrpc.SessionType_TYPE_MACAROON_READONLY, nil

	case session.TypeMacaroonAdmin:
		return litrpc.SessionType_TYPE_MACAROON_ADMIN, nil

	case session.TypeMacaroonCustom:
		return litrpc.SessionType_TYPE_MACAROON_CUSTOM, nil

	case session.TypeUIPassword:
		return litrpc.SessionType_TYPE_UI_PASSWORD, nil

	default:
		return 0, fmt.Errorf("unknown type <%d>", typ)
	}
}

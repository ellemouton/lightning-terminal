package terminal

import (
	"context"
	"fmt"
	"sync"

	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lncfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"google.golang.org/grpc"
	"gopkg.in/macaroon-bakery.v2/bakery"
)

// subServerCfg defines the config values that must be set in order to properly
// define a subServer.
type subServerCfg struct {
	// name is the string identifier of the sub-server.
	name string

	// remote must be set if the sub-server is running remotely and so must
	// not be started in integrated mode.
	remote bool

	// startIntegrated defines how the sub-server should be started if it
	// is to be run in integrated mode.
	startIntegrated func(lnrpc.LightningClient, *lndclient.GrpcLndServices,
		bool) error

	// stopIntegrated defines how the sub-server should be stopped if it is
	// running in integrated mode.
	stopIntegrated func() error

	// registerGrpcService is a function that can be called in order to
	// register the sub-server's rpc with the given registrar.
	registerGrpcService func(grpc.ServiceRegistrar)

	// macValidator will be used to verify any macaroons for requests that
	// are to be handled by the sub-server.
	macValidator macaroons.MacaroonValidator

	// onStartError will be called if there is a startup error (in
	// integrated mode) or if the remote connection could not be made in
	// remote mode.
	onStartError func(err error)

	// onStartSuccess will be called if either the integrated startup func
	// is successful or if the remote connection was successfully made.
	onStartSuccess func()

	// onStop will be called if the subServer is stopped.
	onStop func()

	// ownsURI should return true if the sub-server owns the given request
	// URI.
	ownsURI func(string) bool

	// serverErrChan is an optional error channel that should be listened on
	// after starting the sub-server to listen for any runtime errors.
	serverErrChan chan error

	// remoteCfg is the config that defines what is needed in order to make
	// the remote connection.
	remoteCfg *RemoteDaemonConfig

	// macPath is the path to the sub-server's macaroon if it is not running
	// in remote mode.
	macPath string
}

// subServer defines a LiT subServer that can be run in either integrated or
// remote mode. A subServer is considered non-fatal to LiT meaning that if a
// subServer fails to start, LiT can safetly continue with its operations and
// other subServers can too.
type subServer struct {
	cfg *subServerCfg

	integratedStarted bool
	remoteConn        *grpc.ClientConn

	stopped sync.Once
	quit    chan struct{}
}

// stop the subServer by closing the connection to it if it is remote or by
// stopping the integrated process.
func (s *subServer) stop() error {
	// If the sub-server is running in integrated mode and has not yet
	// started, then we can exit early.
	if !s.cfg.remote && !s.integratedStarted {
		return nil
	}

	var returnErr error
	s.stopped.Do(func() {
		close(s.quit)

		s.cfg.onStop()

		// If running in remote mode, close the connection.
		if s.cfg.remote && s.remoteConn != nil {
			err := s.remoteConn.Close()
			if err != nil {
				returnErr = fmt.Errorf("could not close "+
					"remote connection: %v", err)
			}
			return
		}

		// Else, stop the integrated sub-server process.
		err := s.cfg.stopIntegrated()
		if err != nil {
			returnErr = fmt.Errorf("could not close "+
				"integrated connection: %v", err)
			return
		}

		if s.cfg.serverErrChan == nil {
			return
		}

		returnErr = <-s.cfg.serverErrChan
	})

	return returnErr
}

// startIntegrated starts the subServer in integrated mode.
func (s *subServer) startIntegrated(lndClient lnrpc.LightningClient,
	lndGrpc *lndclient.GrpcLndServices, withMacaroonService bool) {

	err := s.cfg.startIntegrated(lndClient, lndGrpc, withMacaroonService)
	if err != nil {
		s.cfg.onStartError(err)
		return
	}
	s.integratedStarted = true
	s.cfg.onStartSuccess()

	if s.cfg.serverErrChan == nil {
		return
	}

	go func() {
		select {
		case err := <-s.cfg.serverErrChan:
			// The sub server should shut itself down if an error
			// happens. We don't need to try to stop it again.
			s.integratedStarted = false

			s.cfg.onStartError(fmt.Errorf("received "+
				"critical error from sub-server, "+
				"shutting down: %v", err),
			)

		case <-s.quit:
		}
	}()
}

// connectRemote attempts to make a connection to the remote sub-server.
func (s *subServer) connectRemote() {
	var err error
	s.remoteConn, err = dialBackend(
		s.cfg.name, s.cfg.remoteCfg.RPCServer,
		lncfg.CleanAndExpandPath(s.cfg.remoteCfg.TLSCertPath),
	)
	if err != nil {
		err := fmt.Errorf("remote dial error: %v", err)
		s.cfg.onStartError(err)
		return
	}
	s.cfg.onStartSuccess()
}

// newSubServer constructs a new subServer using the given config.
func newSubServer(cfg *subServerCfg) *subServer {
	return &subServer{
		cfg:  cfg,
		quit: make(chan struct{}),
	}
}

// subServerMgr manages a set of subServer objects.
type subServerMgr struct {
	servers []*subServer
	mu      sync.RWMutex
}

// newSubServerMgr constructs a new subServerMgr.
func newSubServerMgr() *subServerMgr {
	return &subServerMgr{
		servers: []*subServer{},
	}
}

// AddServer adds a new subServer to the manager's set.
func (s *subServerMgr) AddServer(server *subServer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.servers = append(s.servers, server)
}

// StartIntegratedServers starts all the manager's sub-servers that should be
// started in integrated mode.
func (s *subServerMgr) StartIntegratedServers(lndClient lnrpc.LightningClient,
	lndGrpc *lndclient.GrpcLndServices, withMacaroonService bool) {

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ss := range s.servers {
		if ss.cfg.remote {
			continue
		}

		ss.startIntegrated(lndClient, lndGrpc, withMacaroonService)
	}
}

// StartRemoteSubServers creates connections to all the manager's sub-servers
// that are running remotely.
func (s *subServerMgr) StartRemoteSubServers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ss := range s.servers {
		if !ss.cfg.remote {
			continue
		}

		ss.connectRemote()
	}
}

// RegisterRPCServices registers all the manager's sub-servers with the given
// grpc registrar.
func (s *subServerMgr) RegisterRPCServices(server grpc.ServiceRegistrar) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ss := range s.servers {
		// In remote mode the "director" of the RPC proxy will act as
		// a catch-all for any gRPC request that isn't known because we
		// didn't register any server for it. The director will then
		// forward the request to the remote service.
		if ss.cfg.remote {
			continue
		}

		ss.cfg.registerGrpcService(server)
	}
}

// GetRemoteConn checks if any of the manager's sub-servers owns the given uri
// and if so, the remote connection to that sub-server is returned. The bool
// return value indicates if the uri is managed by one of the sub-servers
// running in remote mode.
func (s *subServerMgr) GetRemoteConn(uri string) (bool, *grpc.ClientConn) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ss := range s.servers {
		if !ss.cfg.ownsURI(uri) {
			continue
		}

		if !ss.cfg.remote {
			return false, nil
		}

		return true, ss.remoteConn
	}

	return false, nil
}

// ValidateMacaroon checks if any of the manager's sub-servers owns the given
// uri and if so, if it is running in remote mode, then true is returned since
// the macaroon will be validated by the remote subserver itself when the
// request arrives. Otherwise, the integrated sub-server's validator validates
// the macaroon.
func (s *subServerMgr) ValidateMacaroon(ctx context.Context,
	requiredPermissions []bakery.Op, uri string) (bool, error) {

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ss := range s.servers {
		if !ss.cfg.ownsURI(uri) {
			continue
		}

		if ss.cfg.remote {
			return true, nil
		}

		if !ss.integratedStarted {
			return true, fmt.Errorf("%s is not yet ready for "+
				"requests, lnd possibly still starting or "+
				"syncing", ss.cfg.name)
		}

		err := ss.cfg.macValidator.ValidateMacaroon(
			ctx, requiredPermissions, uri,
		)
		if err != nil {
			return true, &proxyErr{
				proxyContext: ss.cfg.name,
				wrapped: fmt.Errorf("invalid macaroon: %v",
					err),
			}
		}
	}

	return false, nil
}

// HandledBy returns true if one of its sub-servers owns the given URI.
func (s *subServerMgr) HandledBy(uri string) (bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ss := range s.servers {
		if !ss.cfg.ownsURI(uri) {
			continue
		}

		return true, ss.cfg.name
	}

	return false, ""
}

// MacaroonPath checks if any of the manager's sub-servers owns the given uri
// and if so, the appropriate macaroon path is returned for that sub-server.
func (s *subServerMgr) MacaroonPath(uri string) (bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ss := range s.servers {
		if !ss.cfg.ownsURI(uri) {
			continue
		}

		if ss.cfg.remote {
			return true, ss.cfg.remoteCfg.MacaroonPath
		}
		return true, ss.cfg.macPath
	}

	return false, ""
}

// ReadRemoteMacaroon checks if any of the manager's sub-servers running in
// remote mode owns the given uri and if so, the appropriate macaroon path is
// returned for that sub-server.
func (s *subServerMgr) ReadRemoteMacaroon(uri string) (bool, []byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ss := range s.servers {
		if !ss.cfg.ownsURI(uri) {
			continue
		}

		if !ss.cfg.remote {
			return false, nil, nil
		}

		macBytes, err := readMacaroon(lncfg.CleanAndExpandPath(
			ss.cfg.remoteCfg.MacaroonPath,
		))

		return true, macBytes, err
	}

	return false, nil, nil
}

// Stop stops all the manager's sub-servers
func (s *subServerMgr) Stop() error {
	var returnErr error

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ss := range s.servers {
		err := ss.stop()
		if err != nil {
			log.Errorf("Error stopping %s: %v", ss.cfg.name, err)
			returnErr = err
		}
	}

	return returnErr
}

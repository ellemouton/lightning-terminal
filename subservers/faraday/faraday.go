package faraday

import (
	"context"

	restProxy "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/lightninglabs/faraday"
	"github.com/lightninglabs/faraday/chain"
	"github.com/lightninglabs/faraday/frdrpc"
	"github.com/lightninglabs/faraday/frdrpcserver"
	"github.com/lightninglabs/faraday/frdrpcserver/perms"
	"github.com/lightninglabs/lightning-terminal/config"
	"github.com/lightninglabs/lightning-terminal/subservers"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lnrpc"
	"google.golang.org/grpc"
	"gopkg.in/macaroon-bakery.v2/bakery"
)

// faradaySubServer implements the SubServer interface.
type faradaySubServer struct {
	*frdrpcserver.RPCServer

	mode string

	cfg       *faraday.Config
	remoteCfg *subservers.RemoteDaemonConfig
}

// A compile-time check to ensure that faradaySubServer implements SubServer.
var _ subservers.SubServer = (*faradaySubServer)(nil)

// newFaradaySubServer returns a new faraday implementation of the SubServer
// interface.
func newFaradaySubServer(cfg *faraday.Config,
	remoteCfg *subservers.RemoteDaemonConfig, mode string) (
	subservers.SubServer, error) {

	subserver := &faradaySubServer{
		mode:      mode,
		cfg:       cfg,
		remoteCfg: remoteCfg,
	}

	var rpcCfg frdrpcserver.Config
	if !subserver.Remote() {
		rpcCfg.FaradayDir = cfg.FaradayDir
		rpcCfg.MacaroonPath = cfg.MacaroonPath

		if cfg.ChainConn {
			var err error
			rpcCfg.BitcoinClient, err = chain.NewBitcoinClient(
				cfg.Bitcoin,
			)
			if err != nil {
				return nil, err
			}
		}
	}

	return &faradaySubServer{
		RPCServer: frdrpcserver.NewRPCServer(&rpcCfg),
		cfg:       cfg,
		remoteCfg: remoteCfg,
		mode:      mode,
	}, nil
}

// Name returns the name of the sub-server.
//
// NOTE: this is part of the SubServer interface.
func (f *faradaySubServer) Name() string {
	return subservers.FARADAY
}

// Remote returns true if the sub-server is running remotely and so
// should be connected to instead of spinning up an integrated server.
//
// NOTE: this is part of the SubServer interface.
func (f *faradaySubServer) Remote() bool {
	return f.mode == config.ModeRemote
}

func (f *faradaySubServer) Enabled() bool {
	return f.mode != config.ModeDisable
}

// RemoteConfig returns the config required to connect to the sub-server
// if it is running in remote mode.
//
// NOTE: this is part of the SubServer interface.
func (f *faradaySubServer) RemoteConfig() *subservers.RemoteDaemonConfig {
	return f.remoteCfg
}

// Start starts the sub-server in integrated mode.
//
// NOTE: this is part of the SubServer interface.
func (f *faradaySubServer) Start(_ lnrpc.LightningClient,
	lndGrpc *lndclient.GrpcLndServices, withMacaroonService bool) error {

	return f.StartAsSubserver(
		lndGrpc.LndServices, withMacaroonService,
	)
}

// RegisterGrpcService must register the sub-server's GRPC server with the given
// registrar.
//
// NOTE: this is part of the SubServer interface.
func (f *faradaySubServer) RegisterGrpcService(service grpc.ServiceRegistrar) {
	frdrpc.RegisterFaradayServerServer(service, f)
}

// RegisterRestService registers the sub-server's REST handlers with the given
// endpoint.
//
// NOTE: this is part of the SubServer interface.
func (f *faradaySubServer) RegisterRestService(ctx context.Context,
	mux *restProxy.ServeMux, endpoint string,
	dialOpts []grpc.DialOption) error {

	return frdrpc.RegisterFaradayServerHandlerFromEndpoint(
		ctx, mux, endpoint, dialOpts,
	)
}

// ServerErrChan returns an error channel that should be listened on after
// starting the sub-server to listen for any runtime errors. It is optional and
// may be set to nil. This only applies in integrated mode.
//
// NOTE: this is part of the SubServer interface.
func (f *faradaySubServer) ServerErrChan() chan error {
	return nil
}

// MacPath returns the path to the sub-server's macaroon if it is not running in
// remote mode.
//
// NOTE: this is part of the SubServer interface.
func (f *faradaySubServer) MacPath() string {
	return f.cfg.MacaroonPath
}

// Permissions returns a map of all RPC methods and their required macaroon
// permissions to access the sub-server.
//
// NOTE: this is part of the SubServer interface.
func (f *faradaySubServer) Permissions() map[string][]bakery.Op {
	return perms.RequiredPermissions
}

// WhiteListedURLs returns a map of all the sub-server's URLs that can be
// accessed without a macaroon.
//
// NOTE: this is part of the SubServer interface.
func (f *faradaySubServer) WhiteListedURLs() map[string]struct{} {
	return nil
}
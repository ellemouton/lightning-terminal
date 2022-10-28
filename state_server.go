package terminal

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/lightninglabs/lightning-terminal/litrpc"
	"google.golang.org/grpc"
)

const (
	LitSubServer     string = "lit"
	LNDSubServer     string = "lnd"
	PoolSubServer    string = "pool"
	LoopSubServer    string = "loop"
	FaradaySubServer string = "faraday"
)

// statusServer is an implementation of the litrpc.StatusServer which can be
// queried for the status of various LiT sub-servers.
type statusServer struct {
	litrpc.UnimplementedStatusServer

	m  map[string]*subServerState
	mu sync.RWMutex

	grpcServer   *grpc.Server
	grpcWebProxy *grpcweb.WrappedGrpcServer
}

// subServerState represents the status of a sub-server.
type subServerState struct {
	running bool
	err     string
}

// newStatusServer constructs a new statusServer.
func newStatusServer() *statusServer {
	s := &statusServer{
		m: make(map[string]*subServerState),
	}

	options := []grpcweb.Option{
		grpcweb.WithWebsockets(true),
		grpcweb.WithWebsocketPingInterval(2 * time.Minute),
		grpcweb.WithCorsForRegisteredEndpointsOnly(false),
	}
	s.grpcServer = grpc.NewServer()
	s.grpcWebProxy = grpcweb.WrapServer(s.grpcServer, options...)

	litrpc.RegisterStatusServer(s.grpcServer, s)

	return s
}

// Stop stops the status server.
func (s *statusServer) Stop() error {
	s.grpcServer.Stop()
	return nil
}

// GetSubServerState queries the current status of a given sub-server.
//
// NOTE: this is part of the litrpc.StatusServer interface.
func (s *statusServer) GetSubServerState(_ context.Context,
	req *litrpc.GetSubServerStateReq) (*litrpc.GetSubServerStateResp,
	error) {

	running, err := s.getSubServerState(req.SubServerName)

	return &litrpc.GetSubServerStateResp{
		Running: running,
		Error:   err,
	}, nil
}

// getSubServerState queries the current status of a given sub-server.
func (s *statusServer) getSubServerState(name string) (bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	system, ok := s.m[name]
	if !ok {
		return false, ""
	}

	return system.running, system.err
}

// setServerRunning can be used to set the status of a sub-server as running
// with no errors.
func (s *statusServer) setServerRunning(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.m[name] = &subServerState{
		running: true,
	}
}

// setServerStopped can be used to set the status of a sub-server as not running
// and with no errors.
func (s *statusServer) setServerStopped(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.m[name] = &subServerState{
		running: false,
	}
}

// setServerErrored can be used to set the status of a sub-server as not running
// and also to set an error message for the sub-server.
func (s *statusServer) setServerErrored(name string, errStr string,
	params ...interface{}) {

	s.mu.Lock()
	defer s.mu.Unlock()

	err := fmt.Sprintf(errStr, params...)
	log.Errorf("could not start the %s sub-server: %s", name, err)

	s.m[name] = &subServerState{
		running: false,
		err:     err,
	}
}

// isHandling checks if the specified request is something to be handled by the
// status server. If true is returned, the call was handled by the status
// server and the caller MUST NOT handle it again. If false is returned, the
// request was not handled and the caller MUST handle it.
func (s *statusServer) isHandling(resp http.ResponseWriter,
	req *http.Request) bool {

	if !strings.Contains(req.URL.Path, "/litrpc.Status") {
		return false
	}

	// gRPC web requests are easy to identify. Send them to the gRPC
	// web proxy.
	if s.grpcWebProxy.IsGrpcWebRequest(req) ||
		s.grpcWebProxy.IsGrpcWebSocketRequest(req) {

		log.Infof("Handling gRPC web request: %s", req.URL.Path)
		s.grpcWebProxy.ServeHTTP(resp, req)

		return true
	}

	// Normal gRPC requests are also easy to identify. These we can
	// send directly to the lnd proxy's gRPC server.
	if isGrpcRequest(req) {
		log.Infof("Handling gRPC request: %s", req.URL.Path)
		s.grpcServer.ServeHTTP(resp, req)

		return true
	}

	return false
}

// A compile-time check to ensure that the statusServer implements the
// litrpc.StatusServer
var _ litrpc.StatusServer = (*statusServer)(nil)

package status

import (
	"context"
	"fmt"
	"sync"

	"github.com/lightninglabs/lightning-terminal/litrpc"
)

// subServerStatus represents the status of a sub-server.
type subServerStatus struct {
	running bool
	err     string
}

// newSubServerStatus constructs a new subServerStatus.
func newSubServerStatus() *subServerStatus {
	return &subServerStatus{}
}

// Manager managers the status of any sub-server registered to it. It is also an
// implementation of the litrpc.StatusServer which can be queried for the status
// of various LiT sub-servers.
type Manager struct {
	litrpc.UnimplementedStatusServer

	subServers map[string]*subServerStatus
	mu         sync.RWMutex
}

// NewStatusManager constructs a new Manager.
func NewStatusManager() *Manager {
	return &Manager{
		subServers: map[string]*subServerStatus{},
	}
}

// RegisterSubServer will create a new sub-server entry for the Manager to
// keep track of.
func (s *Manager) RegisterSubServer(name string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.subServers[name] = newSubServerStatus()
}

// SubServerStatus queries the current status of a given sub-server.
//
// NOTE: this is part of the litrpc.StatusServer interface.
func (s *Manager) SubServerStatus(_ context.Context,
	_ *litrpc.SubServerStatusReq) (*litrpc.SubServerStatusResp,
	error) {

	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := make(map[string]*litrpc.SubServerStatus, len(s.subServers))
	for server, status := range s.subServers {
		resp[server] = &litrpc.SubServerStatus{
			Running: status.running,
			Error:   status.err,
		}
	}

	return &litrpc.SubServerStatusResp{
		SubServers: resp,
	}, nil
}

// GetStatus queries the current status of a given sub-server.
func (s *Manager) GetStatus(name string) (bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	system, ok := s.subServers[name]
	if !ok {
		return false, ""
	}

	return system.running, system.err
}

// SetRunning can be used to set the status of a sub-server as running
// with no errors.
func (s *Manager) SetRunning(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subServers[name] = &subServerStatus{
		running: true,
	}
}

// SetStopped can be used to set the status of a sub-server as not running and
// with no errors.
func (s *Manager) SetStopped(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subServers[name] = &subServerStatus{
		running: false,
	}
}

// SetErrored can be used to set the status of a sub-server as not running
// and also to set an error message for the sub-server.
func (s *Manager) SetErrored(name string, errStr string,
	params ...interface{}) {

	s.mu.Lock()
	defer s.mu.Unlock()

	err := fmt.Sprintf(errStr, params...)
	log.Errorf("could not start the %s sub-server: %s", name, err)

	s.subServers[name] = &subServerStatus{
		running: false,
		err:     err,
	}
}

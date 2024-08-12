package subservers

import (
	"fmt"
	"sync"
)

var (
	servers   = make(map[string]*SubServerDriver)
	serversMu sync.Mutex
)

type SubServerDriver struct {
	// SubServerName is the full name of a sub-sever.
	//
	// NOTE: This MUST be unique.
	SubServerName string

	InitSubServer InitSubServer
}

func RegisterSubServer(driver *SubServerDriver) error {
	serversMu.Lock()
	defer serversMu.Unlock()

	if _, ok := servers[driver.SubServerName]; ok {
		return fmt.Errorf("subserver already registered")
	}

	servers[driver.SubServerName] = driver

	return nil
}

// RegisteredSubServers returns all registered sub-servers.
func RegisteredSubServers() map[string]*SubServerDriver {
	serversMu.Lock()
	defer serversMu.Unlock()

	return servers
}

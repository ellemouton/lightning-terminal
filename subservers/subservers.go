package subservers

import (
	"fmt"
	"sync"
)

var (
	servers   = make(map[Name]*SubServerDriver)
	serversMu sync.Mutex
)

type SubServerDriver struct {
	// SubServerName is the full name of a sub-sever.
	//
	// NOTE: This MUST be unique.
	SubServerName Name

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
//
// NOTE: This function is safe for concurrent access.
func RegisteredSubServers() []*SubServerDriver {
	serversMu.Lock()
	defer serversMu.Unlock()

	drivers := make([]*SubServerDriver, 0, len(servers))
	for _, driver := range servers {
		drivers = append(drivers, driver)
	}

	return drivers
}

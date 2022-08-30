package terminal

import (
	"net"
	"strings"

	faraday "github.com/lightninglabs/faraday/frdrpcserver/perms"
	loop "github.com/lightninglabs/loop/loopd/perms"
	pool "github.com/lightninglabs/pool/perms"
	"github.com/lightningnetwork/lnd"
	"github.com/lightningnetwork/lnd/autopilot"
	"github.com/lightningnetwork/lnd/chainreg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/autopilotrpc"
	"github.com/lightningnetwork/lnd/lnrpc/chainrpc"
	"github.com/lightningnetwork/lnd/lnrpc/devrpc"
	"github.com/lightningnetwork/lnd/lnrpc/invoicesrpc"
	"github.com/lightningnetwork/lnd/lnrpc/neutrinorpc"
	"github.com/lightningnetwork/lnd/lnrpc/peersrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/lnrpc/signrpc"
	"github.com/lightningnetwork/lnd/lnrpc/walletrpc"
	"github.com/lightningnetwork/lnd/lnrpc/watchtowerrpc"
	"github.com/lightningnetwork/lnd/lnrpc/wtclientrpc"
	"github.com/lightningnetwork/lnd/lntest/mock"
	"github.com/lightningnetwork/lnd/routing"
	"github.com/lightningnetwork/lnd/sweep"
	"gopkg.in/macaroon-bakery.v2/bakery"
)

var (
	// litPermissions is a map of all LiT RPC methods and their required
	// macaroon permissions to access the session service.
	litPermissions = map[string][]bakery.Op{
		"/litrpc.Sessions/AddSession": {{
			Entity: "sessions",
			Action: "write",
		}},
		"/litrpc.Sessions/ListSessions": {{
			Entity: "sessions",
			Action: "read",
		}},
		"/litrpc.Sessions/RevokeSession": {{
			Entity: "sessions",
			Action: "write",
		}},
	}

	// whiteListedMethods is a map of all lnd RPC methods that don't require
	// any macaroon authentication.
	whiteListedMethods = map[string][]bakery.Op{
		"/lnrpc.WalletUnlocker/GenSeed":        {},
		"/lnrpc.WalletUnlocker/InitWallet":     {},
		"/lnrpc.WalletUnlocker/UnlockWallet":   {},
		"/lnrpc.WalletUnlocker/ChangePassword": {},

		// The State service must be available at all times, even
		// before we can check macaroons, so we whitelist it.
		"/lnrpc.State/SubscribeState": {},
		"/lnrpc.State/GetState":       {},
	}

	// lndSubServerNameToTag is a map from the name of an LND subserver to
	// the name of the LND tag that corresponds to the subserver. This map
	// only contains the subserver-to-tag pairs for the pairs where the
	// names differ.
	lndSubServerNameToTag = map[string]string{
		"WalletKitRPC":        "walletrpc",
		"DevRPC":              "dev",
		"NeutrinoKitRPC":      "neutrinorpc",
		"VersionRPC":          "verrpc",
		"WatchtowerClientRPC": "wtclientrpc",
	}

	// lndSubServerPerms is a map from LND subserver name to permissions
	// map.
	lndSubServerPerms = make(map[string]map[string][]bakery.Op)
)

// getSubserverPermissions returns a merged map of all subserver macaroon
// permissions.
func getSubserverPermissions() map[string][]bakery.Op {
	mapSize := len(litPermissions) + len(faraday.RequiredPermissions) +
		len(loop.RequiredPermissions) + len(pool.RequiredPermissions)

	result := make(map[string][]bakery.Op, mapSize)
	for key, value := range faraday.RequiredPermissions {
		result[key] = value
	}
	for key, value := range loop.RequiredPermissions {
		result[key] = value
	}
	for key, value := range pool.RequiredPermissions {
		result[key] = value
	}
	for key, value := range litPermissions {
		result[key] = value
	}
	return result
}

// getAllMethodPermissions returns a merged map of lnd's and all subservers'
// method macaroon permissions.
func getAllMethodPermissions(
	lndBuildTags map[string]bool) map[string][]bakery.Op {

	litsubServerPerms := getSubserverPermissions()
	lndPerms := lnd.MainRPCServerPermissions()

	mapSize := len(litsubServerPerms) + len(lndSubServerPerms) +
		len(lndPerms) + len(whiteListedMethods)

	result := make(map[string][]bakery.Op, mapSize)

	for key, value := range lndPerms {
		result[key] = value
	}

	for key, value := range whiteListedMethods {
		result[key] = value
	}

	for key, value := range litsubServerPerms {
		result[key] = value
	}

	for subServerName, perms := range lndSubServerPerms {
		name := subServerName
		if tagName, ok := lndSubServerNameToTag[name]; ok {
			name = tagName
		}

		if !lndBuildTags[strings.ToLower(name)] {
			continue
		}

		for key, value := range perms {
			result[key] = value
		}
	}

	return result
}

// GetAllPermissions retrieves all the permissions needed to bake a super
// macaroon.
func GetAllPermissions(readOnly bool,
	lndBuildTags map[string]bool) []bakery.Op {

	dedupMap := make(map[string]map[string]bool)
	for _, methodPerms := range getAllMethodPermissions(lndBuildTags) {
		for _, methodPerm := range methodPerms {
			if methodPerm.Action == "" || methodPerm.Entity == "" {
				continue
			}

			if readOnly && methodPerm.Action != "read" {
				continue
			}

			if dedupMap[methodPerm.Entity] == nil {
				dedupMap[methodPerm.Entity] = make(
					map[string]bool,
				)
			}
			dedupMap[methodPerm.Entity][methodPerm.Action] = true
		}
	}

	result := make([]bakery.Op, 0, len(dedupMap))
	for entity, actions := range dedupMap {
		for action := range actions {
			result = append(result, bakery.Op{
				Entity: entity,
				Action: action,
			})
		}
	}

	return result
}

// isLndURI returns true if the given URI belongs to an RPC of lnd.
func isLndURI(uri string) bool {
	_, ok := lndSubServerPerms[uri]
	return ok
}

// isLoopURI returns true if the given URI belongs to an RPC of loopd.
func isLoopURI(uri string) bool {
	_, ok := loop.RequiredPermissions[uri]
	return ok
}

// isFaradayURI returns true if the given URI belongs to an RPC of faraday.
func isFaradayURI(uri string) bool {
	_, ok := faraday.RequiredPermissions[uri]
	return ok
}

// isPoolURI returns true if the given URI belongs to an RPC of poold.
func isPoolURI(uri string) bool {
	_, ok := pool.RequiredPermissions[uri]
	return ok
}

// isLitURI returns true if the given URI belongs to an RPC of LiT.
func isLitURI(uri string) bool {
	_, ok := litPermissions[uri]
	return ok
}

func init() {
	ss := lnrpc.RegisteredSubServers()
	for _, subServer := range ss {
		_, perms, err := subServer.NewGrpcHandler().CreateSubServer(
			&mockConfig{},
		)
		if err != nil {
			panic(err)
		}

		name := subServer.SubServerName
		lndSubServerPerms[name] = make(map[string][]bakery.Op)
		for key, value := range perms {
			lndSubServerPerms[name][key] = value
		}
	}
}

// mockConfig implements lnrpc.SubServerConfigDispatcher. It provides th
// functionality required so that the lnrpc.GrpcHandler.CreateSubServer
// function can be called without panicking.
type mockConfig struct{}

var _ lnrpc.SubServerConfigDispatcher = (*mockConfig)(nil)

// FetchConfig is a mock implementation of lnrpc.SubServerConfigDispatcher. It
// is used as a parameter to lnrpc.GrpcHandler.CreateSubServer and allows the
// function to be called without panicking. This is useful because
// CreateSubServer can be used to extract the permissions required by each
// registered subserver.
//
// TODO(elle): remove this once the sub-server permission lists in LND have been
// exported
func (t *mockConfig) FetchConfig(subServerName string) (interface{}, bool) {
	switch subServerName {
	case "InvoicesRPC":
		return &invoicesrpc.Config{}, true
	case "WatchtowerClientRPC":
		return &wtclientrpc.Config{
			Resolver: func(_, _ string) (*net.TCPAddr, error) {
				return nil, nil
			},
		}, true
	case "AutopilotRPC":
		return &autopilotrpc.Config{
			Manager: &autopilot.Manager{},
		}, true
	case "ChainRPC":
		return &chainrpc.Config{
			ChainNotifier: &chainreg.NoChainBackend{},
		}, true
	case "DevRPC":
		return &devrpc.Config{}, true
	case "NeutrinoKitRPC":
		return &neutrinorpc.Config{}, true
	case "PeersRPC":
		return &peersrpc.Config{}, true
	case "RouterRPC":
		return &routerrpc.Config{
			Router: &routing.ChannelRouter{},
		}, true
	case "SignRPC":
		return &signrpc.Config{
			Signer: &mock.DummySigner{},
		}, true
	case "WalletKitRPC":
		return &walletrpc.Config{
			FeeEstimator: &chainreg.NoChainBackend{},
			Wallet:       &mock.WalletController{},
			KeyRing:      &mock.SecretKeyRing{},
			Sweeper:      &sweep.UtxoSweeper{},
			Chain:        &mock.ChainIO{},
		}, true
	case "WatchtowerRPC":
		return &watchtowerrpc.Config{}, true
	default:
		return nil, false
	}
}

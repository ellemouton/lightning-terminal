package itest

import (
	"context"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/lightninglabs/lightning-terminal/litrpc"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"github.com/stretchr/testify/require"
)

// testModeIntegrated makes sure that in integrated mode all daemons work
// correctly. It tests the full integrated mode test suite with the ui password
// set and then again with no ui password and a disabled UI.
func testModeRemote(ctx context.Context, net *NetworkHarness,
	t *harnessTest) {

	testWithAndWithoutUIPassword(ctx, net, t.t, remoteTestSuite, net.Bob)

	testDisablingSubServers(ctx, net, t.t, remoteTestSuite, net.Bob)
}

// remoteTestSuite makes sure that in remote mode all daemons work correctly.
func remoteTestSuite(ctx context.Context, net *NetworkHarness, t *testing.T,
	withoutUIPassword, subServersDisabled bool, runNum int) {

	// Some very basic functionality tests to make sure lnd is working fine
	// in remote mode.
	net.LNDHarness.FundCoins(btcutil.SatoshiPerBitcoin, net.Bob.RemoteLnd)

	// We expect a non-empty alias (truncated node ID) to be returned.
	resp, err := net.Bob.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Alias)
	require.Contains(t, resp.Alias, "0")

	t.Run("certificate check", func(tt *testing.T) {
		cfg := net.Bob.Cfg

		// In remote mode we expect the LiT HTTPS port (8443 by default)
		// and to have its own certificate
		litCerts, err := getServerCertificates(cfg.LitAddr())
		require.NoError(tt, err)
		require.Len(tt, litCerts, 1)
		require.Equal(
			tt, "litd autogenerated cert",
			litCerts[0].Issuer.Organization[0],
		)
	})

	t.Run("gRPC macaroon auth check", func(tt *testing.T) {
		cfg := net.Bob.Cfg

		for _, endpoint := range endpoints {
			endpoint := endpoint
			endpointEnabled := subServersDisabled &&
				endpoint.canDisable

			tt.Run(endpoint.name+" lit port", func(ttt *testing.T) {
				runGRPCAuthTest(
					ttt, cfg.LitAddr(), cfg.LitTLSCertPath,
					endpoint.macaroonFn(cfg),
					endpoint.noAuth,
					endpoint.requestFn,
					endpoint.successPattern,
					endpointEnabled,
					endpoint.disabledPattern,
				)
			})
		}
	})

	t.Run("UI password auth check", func(tt *testing.T) {
		cfg := net.Bob.Cfg

		for _, endpoint := range endpoints {
			endpoint := endpoint
			endpointEnabled := subServersDisabled &&
				endpoint.canDisable

			shouldFailWithoutMacaroon := false
			if withoutUIPassword {
				shouldFailWithoutMacaroon = true
			}

			tt.Run(endpoint.name+" lit port", func(ttt *testing.T) {
				runUIPasswordCheck(
					ttt, cfg.LitAddr(), cfg.LitTLSCertPath,
					cfg.UIPassword, endpoint.requestFn,
					endpoint.noAuth,
					shouldFailWithoutMacaroon,
					endpoint.successPattern,
					endpointEnabled,
					endpoint.disabledPattern,
				)
			})
		}
	})

	t.Run("UI index page fallback", func(tt *testing.T) {
		runIndexPageCheck(tt, net.Bob.Cfg.LitAddr(), withoutUIPassword)
	})

	t.Run("grpc-web auth", func(tt *testing.T) {
		cfg := net.Bob.Cfg

		for _, endpoint := range endpoints {
			endpoint := endpoint
			endpointEnabled := subServersDisabled &&
				endpoint.canDisable

			tt.Run(endpoint.name+" lit port", func(ttt *testing.T) {
				runGRPCWebAuthTest(
					ttt, cfg.LitAddr(), cfg.UIPassword,
					endpoint.grpcWebURI, withoutUIPassword,
					endpointEnabled,
					endpoint.disabledPattern,
					endpoint.noAuth,
				)
			})
		}
	})

	t.Run("gRPC super macaroon auth check", func(tt *testing.T) {
		cfg := net.Bob.Cfg

		superMacFile := bakeSuperMacaroon(tt, cfg, true)

		for _, endpoint := range endpoints {
			endpoint := endpoint
			endpointEnabled := subServersDisabled &&
				endpoint.canDisable

			tt.Run(endpoint.name+" lit port", func(ttt *testing.T) {
				runGRPCAuthTest(
					ttt, cfg.LitAddr(), cfg.LitTLSCertPath,
					superMacFile, endpoint.noAuth,
					endpoint.requestFn,
					endpoint.successPattern,
					endpointEnabled,
					endpoint.disabledPattern,
				)
			})
		}
	})

	t.Run("REST auth", func(tt *testing.T) {
		cfg := net.Bob.Cfg

		for _, endpoint := range endpoints {
			endpoint := endpoint
			endpointDisabled := subServersDisabled &&
				endpoint.canDisable

			tt.Run(endpoint.name+" lit port", func(ttt *testing.T) {
				runRESTAuthTest(
					ttt, cfg.LitAddr(), cfg.UIPassword,
					endpoint.macaroonFn(cfg),
					endpoint.restWebURI,
					endpoint.successPattern,
					endpoint.restPOST, withoutUIPassword,
					endpointDisabled, endpoint.noAuth,
				)
			})
		}
	})

	t.Run("lnc auth", func(tt *testing.T) {
		cfg := net.Bob.Cfg

		ctx := context.Background()
		ctxt, cancel := context.WithTimeout(ctx, defaultTimeout)
		defer cancel()

		rawLNCConn := setUpLNCConn(
			ctxt, tt, cfg.LitAddr(), cfg.LitTLSCertPath,
			cfg.LitMacPath,
			litrpc.SessionType_TYPE_MACAROON_READONLY, nil,
		)
		defer rawLNCConn.Close()

		for _, endpoint := range endpoints {
			endpoint := endpoint
			endpointDisabled := subServersDisabled &&
				endpoint.canDisable

			tt.Run(endpoint.name+" lit port", func(ttt *testing.T) {
				runLNCAuthTest(
					ttt, rawLNCConn, endpoint.requestFn,
					endpoint.successPattern,
					endpoint.allowedThroughLNC,
					"unknown service",
					endpointDisabled,
					endpoint.disabledPattern,
					endpoint.noAuth,
				)
			})
		}
	})

	t.Run("lnc auth custom mac perms", func(tt *testing.T) {
		cfg := net.Bob.Cfg

		ctx := context.Background()
		ctxt, cancel := context.WithTimeout(ctx, defaultTimeout)
		defer cancel()

		customPerms := make(
			[]*litrpc.MacaroonPermission, 0, len(customURIs),
		)

		customURIKeyword := macaroons.PermissionEntityCustomURI
		for uri := range customURIs {
			customPerms = append(
				customPerms, &litrpc.MacaroonPermission{
					Entity: customURIKeyword,
					Action: uri,
				},
			)
		}

		rawLNCConn := setUpLNCConn(
			ctxt, tt, cfg.LitAddr(), cfg.LitTLSCertPath,
			cfg.LitMacPath,
			litrpc.SessionType_TYPE_MACAROON_CUSTOM, customPerms,
		)
		defer rawLNCConn.Close()

		for _, endpoint := range endpoints {
			endpoint := endpoint
			endpointDisabled := subServersDisabled &&
				endpoint.canDisable

			expectedErr := "permission denied"
			if endpoint.noAuth {
				expectedErr = "unknown service"
			}

			tt.Run(endpoint.name+" lit port", func(ttt *testing.T) {
				allowed := customURIs[endpoint.grpcWebURI]
				runLNCAuthTest(
					ttt, rawLNCConn, endpoint.requestFn,
					endpoint.successPattern,
					allowed, expectedErr,
					endpointDisabled,
					endpoint.disabledPattern,
					endpoint.noAuth,
				)
			})
		}
	})

	t.Run("gRPC super macaroon account system test", func(tt *testing.T) {
		cfg := net.Bob.Cfg

		// If the accounts service is disabled, we skip this test as it
		// will fail due to the accounts service being disabled.
		if subServersDisabled {
			return
		}

		superMacFile := bakeSuperMacaroon(tt, cfg, false)

		ht := newHarnessTest(tt, net)
		runAccountSystemTest(
			ht, net.Bob, cfg.LitAddr(), cfg.LitTLSCertPath,
			superMacFile, runNum,
		)
	})
}

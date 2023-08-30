package rules

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/lightninglabs/lightning-terminal/firewall/mock"
	"github.com/lightninglabs/lightning-terminal/firewalldb"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/stretchr/testify/require"
)

// TestChannelRestrictCheckRequest ensures that the ChannelRestrictEnforcer
// correctly accepts or denys a request.
func TestChannelRestrictCheckRequest(t *testing.T) {
	txid1, index1, err := newTXID()
	require.NoError(t, err)

	txid2, index2, err := newTXID()
	require.NoError(t, err)

	txid3, index3, err := newTXID()
	require.NoError(t, err)

	chanPointStr1 := fmt.Sprintf("%s:%d", hex.EncodeToString(txid1), index1)
	chanPointStr2 := fmt.Sprintf("%s:%d", hex.EncodeToString(txid2), index2)
	chanPointStr3 := fmt.Sprintf("%s:%d", hex.EncodeToString(txid3), index3)

	chanID1, _ := firewalldb.NewPseudoUint64()
	chanID2, _ := firewalldb.NewPseudoUint64()
	chanID3, _ := firewalldb.NewPseudoUint64()

	ctx := context.Background()
	mgr := NewChannelRestrictMgr()
	cfg := &mockLndClient{
		channels: []lndclient.ChannelInfo{
			{
				ChannelID:    chanID1,
				ChannelPoint: chanPointStr1,
			},
			{
				ChannelID:    chanID2,
				ChannelPoint: chanPointStr2,
			},
			{
				ChannelID:    chanID3,
				ChannelPoint: chanPointStr3,
			},
		},
	}
	enf, err := mgr.NewEnforcer(cfg, &ChannelRestrict{
		DenyList: []uint64{
			chanID1, chanID2,
		},
	})
	require.NoError(t, err)

	// A request for an irrelevant URI should be allowed.
	_, err = enf.HandleRequest(ctx, "random-URI", nil)
	require.NoError(t, err)

	// If there is a channel restriction list, then no global policy updates
	// are allowed.
	_, err = enf.HandleRequest(
		ctx, "/lnrpc.Lightning/UpdateChannelPolicy",
		&lnrpc.PolicyUpdateRequest{
			Scope: &lnrpc.PolicyUpdateRequest_Global{Global: true},
		},
	)
	require.ErrorContainsf(t, err, "cant apply call to global scope when "+
		"using a channel restriction list", "")

	// Test that an action on channel point 1 in the string form is
	// disallowed.
	chanPoint1 := &lnrpc.ChannelPoint{
		FundingTxid: &lnrpc.ChannelPoint_FundingTxidStr{
			FundingTxidStr: hex.EncodeToString(txid1),
		},
		OutputIndex: index1,
	}

	_, err = enf.HandleRequest(
		ctx, "/lnrpc.Lightning/UpdateChannelPolicy",
		&lnrpc.PolicyUpdateRequest{
			Scope: &lnrpc.PolicyUpdateRequest_ChanPoint{
				ChanPoint: chanPoint1,
			},
		},
	)
	require.ErrorContainsf(t, err, "illegal action on channel in channel "+
		"restriction list", "")

	// Test that an action on channel point 2 in the byte form is
	// disallowed.
	h, err := chainhash.NewHashFromStr(hex.EncodeToString(txid2))
	require.NoError(t, err)

	chanPoint2 := &lnrpc.ChannelPoint{
		FundingTxid: &lnrpc.ChannelPoint_FundingTxidBytes{
			FundingTxidBytes: h[:],
		},
		OutputIndex: index2,
	}

	_, err = enf.HandleRequest(
		ctx, "/lnrpc.Lightning/UpdateChannelPolicy",
		&lnrpc.PolicyUpdateRequest{
			Scope: &lnrpc.PolicyUpdateRequest_ChanPoint{
				ChanPoint: chanPoint2,
			},
		},
	)
	require.ErrorContainsf(t, err, "illegal action on channel in channel "+
		"restriction list", "")

	// Test that an action on a channel not in the deny-list is allowed.
	chanPoint3 := &lnrpc.ChannelPoint{
		FundingTxid: &lnrpc.ChannelPoint_FundingTxidStr{
			FundingTxidStr: hex.EncodeToString(txid3),
		},
		OutputIndex: index3,
	}

	_, err = enf.HandleRequest(
		ctx, "/lnrpc.Lightning/UpdateChannelPolicy",
		&lnrpc.PolicyUpdateRequest{
			Scope: &lnrpc.PolicyUpdateRequest_ChanPoint{
				ChanPoint: chanPoint3,
			},
		},
	)
	require.NoError(t, err)
}

func newTXID() ([]byte, uint32, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, 0, err
	}

	return b[:], rand.Uint32(), nil
}

type mockLndClient struct {
	lndclient.LightningClient
	Config

	channels []lndclient.ChannelInfo
}

func (m *mockLndClient) GetLndClient() lndclient.LightningClient {
	return m
}

func (m *mockLndClient) ListChannels(_ context.Context, _, _ bool) (
	[]lndclient.ChannelInfo, error) {

	return m.channels, nil
}

// TestChannelRestrictRealToPseudo tests that the ChannelRestrict's RealToPseudo
// method correctly determines which real strings to generate pseudo pairs for
// based on the privacy map db passed to it.
func TestChannelRestrictRealToPseudo(t *testing.T) {
	chanID1 := firewalldb.Uint64ToStr(1)
	chanID2 := firewalldb.Uint64ToStr(2)
	chanID3 := firewalldb.Uint64ToStr(3)
	chanID2Obfuscated := firewalldb.Uint64ToStr(200)

	tests := []struct {
		name           string
		dbPreLoad      map[string]string
		expectNewPairs map[string]bool
	}{
		{
			// If there is no preloaded DB, then we expect all the
			// values in the deny list to be returned from the
			// RealToPseudo method.
			name: "no pre loaded db",
			expectNewPairs: map[string]bool{
				chanID1: true,
				chanID2: true,
				chanID3: true,
			},
		},
		{
			// If the DB is preloaded with an entry for "channel 2"
			// then we don't expect that entry to be returned in the
			// set of new pairs.
			name: "partially pre-loaded DB",
			dbPreLoad: map[string]string{
				chanID2: chanID2Obfuscated,
			},
			expectNewPairs: map[string]bool{
				chanID1: true,
				chanID3: true,
			},
		},
	}

	cr := &ChannelRestrict{
		DenyList: []uint64{
			1,
			2,
			3,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var privDB firewalldb.PrivacyMapDB
			if len(test.dbPreLoad) != 0 {
				privDB = mock.NewPrivacyMapDB()
			}

			var expectedDenyList []string
			for r, p := range test.dbPreLoad {
				err := privDB.View(
					func(tx firewalldb.PrivacyMapTx) error {
						return tx.NewPair(r, p)
					},
				)
				require.NoError(t, err)

				expectedDenyList = append(expectedDenyList, p)
			}

			v, newPairs, err := cr.RealToPseudo(privDB)
			require.NoError(t, err)
			require.Len(t, newPairs, len(test.expectNewPairs))

			for r, p := range newPairs {
				require.True(t, test.expectNewPairs[r])

				expectedDenyList = append(expectedDenyList, p)
			}

			denyList, ok := v.(*ChannelRestrict)
			require.True(t, ok)

			require.EqualValues(t, v, denyList)
		})
	}
}

package rules

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/lightninglabs/lightning-terminal/firewall/mock"
	"github.com/lightninglabs/lightning-terminal/firewalldb"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/stretchr/testify/require"
)

// TestPeerRestrictCheckRequest ensures that the PeerRestrictEnforcer correctly
// accepts or denys a request.
func TestPeerRestrictCheckRequest(t *testing.T) {
	txid1, index1, err := newTXID()
	require.NoError(t, err)

	txid2, index2, err := newTXID()
	require.NoError(t, err)

	txid3, index3, err := newTXID()
	require.NoError(t, err)

	chanPointStr1 := fmt.Sprintf("%s:%d", hex.EncodeToString(txid1), index1)
	chanPointStr2 := fmt.Sprintf("%s:%d", hex.EncodeToString(txid2), index2)
	chanPointStr3 := fmt.Sprintf("%s:%d", hex.EncodeToString(txid3), index3)

	peerID1, err := firewalldb.NewPseudoStr(66)
	require.NoError(t, err)

	peerID2, err := firewalldb.NewPseudoStr(66)
	require.NoError(t, err)

	peerID3, err := firewalldb.NewPseudoStr(66)
	require.NoError(t, err)

	peerKey1, err := route.NewVertexFromStr(peerID1)
	require.NoError(t, err)

	peerKey2, err := route.NewVertexFromStr(peerID2)
	require.NoError(t, err)

	peerKey3, err := route.NewVertexFromStr(peerID3)
	require.NoError(t, err)

	ctx := context.Background()
	mgr := NewPeerRestrictMgr()
	cfg := &mockLndClient{
		channels: []lndclient.ChannelInfo{
			{
				ChannelPoint: chanPointStr1,
				PubKeyBytes:  peerKey1,
			},
			{
				ChannelPoint: chanPointStr2,
				PubKeyBytes:  peerKey2,
			},
			{
				ChannelPoint: chanPointStr3,
				PubKeyBytes:  peerKey3,
			},
		},
	}

	enf, err := mgr.NewEnforcer(cfg, &PeerRestrict{
		DenyList: []string{
			peerID1, peerID2,
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
	require.ErrorContainsf(t, err, "cant apply call to global scope "+
		"when using a peer restriction list", "")

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
	require.ErrorContainsf(t, err, "illegal action on peer in peer "+
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
	require.ErrorContainsf(t, err, "illegal action on peer in peer "+
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

// TestPeerRestrictionRealToPseudo tests that the PeerRestriction's RealToPseudo
// method correctly determines which real strings to generate pseudo pairs for
// based on the privacy map db passed to it.
func TestPeerRestrictRealToPseudo(t *testing.T) {
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
				"peer 1": true,
				"peer 2": true,
				"peer 3": true,
			},
		},
		{
			// If the DB is preloaded with an entry for "peer 2"
			// then we don't expect that entry to be returned in the
			// set of new pairs.
			name: "partially pre-loaded DB",
			dbPreLoad: map[string]string{
				"peer 2": "obfuscated peer 2",
			},
			expectNewPairs: map[string]bool{
				"peer 1": true,
				"peer 3": true,
			},
		},
	}

	pr := &PeerRestrict{
		DenyList: []string{
			"peer 1",
			"peer 2",
			"peer 3",
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

			v, newPairs, err := pr.RealToPseudo(privDB)
			require.NoError(t, err)
			require.Len(t, newPairs, len(test.expectNewPairs))

			for r, p := range newPairs {
				require.True(t, test.expectNewPairs[r])

				expectedDenyList = append(expectedDenyList, p)
			}

			denyList, ok := v.(*PeerRestrict)
			require.True(t, ok)

			require.EqualValues(t, v, denyList)
		})
	}
}

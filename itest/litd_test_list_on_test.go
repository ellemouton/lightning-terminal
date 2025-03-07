//go:build itest
// +build itest

package itest

import (
	"fmt"
)

var allTestCases = []*testCase{
	{
		name: "stateless init mode",
		test: testStatelessInitMode,
	},
	{
		name: "test firewall rules",
		test: testFirewallRules,
	},
	{
		name: "test large http header",
		test: testLargeHttpHeader,
	},
	{
		name: "test custom channels",
		test: testCustomChannels,
	},
	{
		name: "test custom channels large",
		test: testCustomChannelsLarge,
	},
	{
		name: "test custom channels grouped asset",
		test: testCustomChannelsGroupedAsset,
	},
	{
		name: "test custom channels force close",
		test: testCustomChannelsForceClose,
	},
	{
		name: "test custom channels breach",
		test: testCustomChannelsBreach,
	},
	{
		name: "test custom channels liquidity",
		test: testCustomChannelsLiquidityEdgeCases,
	},
	{
		name: "test custom channels htlc force close",
		test: testCustomChannelsHtlcForceClose,
	},
	{
		name: "test custom channels balance consistency",
		test: testCustomChannelsBalanceConsistency,
	},
	{
		name: "test custom channels single asset multi input",
		test: testCustomChannelsSingleAssetMultiInput,
	},
	{
		name: "test custom channels oracle pricing",
		test: testCustomChannelsOraclePricing,
	},
	{
		name: "test custom channels fee",
		test: testCustomChannelsFee,
	},
	{
		name: "test custom channels forward bandwidth",
		test: testCustomChannelsForwardBandwidth,
	},
	{
		name: "test custom channels strict forwarding",
		test: testCustomChannelsStrictForwarding,
	},
	{
		name: "test custom channels decode payreq",
		test: testCustomChannelsDecodeAssetInvoice,
	},
}

// appendPrefixed is used to add a prefix to each test name in the subtests
// before appending them to the main test cases.
func appendPrefixed(prefix string, testCases,
	subtestCases []*testCase) []*testCase {

	for _, tc := range subtestCases {
		name := fmt.Sprintf("%s-%s", prefix, tc.name)
		testCases = append(testCases, &testCase{
			name: name,
			test: tc.test,
		})
	}

	return testCases
}

func init() {
	// Register subtests.
	allTestCases = appendPrefixed(
		"mode integrated", allTestCases, integratedModeTestCases,
	)
	allTestCases = appendPrefixed(
		"mode remote", allTestCases, remoteModeTestCases,
	)
}

package accounts

import (
	"context"
	"testing"
	"time"

	"github.com/lightninglabs/taproot-assets/fn"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/stretchr/testify/require"
)

// TestAccountsStore tests the basic functionality of the accounts store using
// different backends.
func TestAccountsStore(t *testing.T) {
	time.Local = time.UTC

	testList := []struct {
		name string
		test func(t *testing.T, makeDB func(t *testing.T) Store)
	}{
		{
			name: "BasicAccountStorage",
			test: testBasicAccountStorage,
		},
		{
			name: "LastInvoiceIndexes",
			test: testLastInvoiceIndexes,
		},
	}

	for _, test := range testList {
		for _, db := range dbImpls {
			t.Run(test.name+"_"+db.name, func(t *testing.T) {
				test.test(t, db.makeDB)
			})
		}
	}
}

// testBasicAccountStorage tests that accounts can be stored and retrieved correctly.
func testBasicAccountStorage(t *testing.T, makeDB func(t *testing.T) Store) {
	t.Parallel()
	ctx := context.Background()

	store := makeDB(t)

	// Create an account that does not expire.
	acct1, err := store.NewAccount(ctx, 0, time.Time{}, "foo")
	require.NoError(t, err)
	require.False(t, acct1.HasExpired())

	dbAccount, err := store.Account(ctx, acct1.ID)
	require.NoError(t, err)

	assertEqualAccounts(t, acct1, dbAccount)

	// Make sure we cannot create a second account with the same label.
	_, err = store.NewAccount(ctx, 123, time.Time{}, "foo")
	require.ErrorContains(t, err, "account with the label 'foo' already")

	// Make sure we cannot set a label that looks like an account ID.
	_, err = store.NewAccount(ctx, 123, time.Time{}, "0011223344556677")
	require.ErrorContains(t, err, "is not allowed as it can be mistaken")

	now := time.Now()

	// Update all values of the account that we can modify.
	err = store.UpdateAccountBalanceAndExpiry(
		ctx, acct1.ID, fn.Some(int64(-500)), fn.Some(now),
	)
	require.NoError(t, err)
	err = store.AddAccountPayment(
		ctx, acct1.ID, lntypes.Hash{12, 34, 56, 78}, 123456,
	)
	require.NoError(t, err)
	err = store.UpdateAccountPaymentStatus(
		ctx, acct1.ID, lntypes.Hash{12, 34, 56, 78},
		lnrpc.Payment_FAILED,
	)
	require.NoError(t, err)
	err = store.AddAccountPayment(
		ctx, acct1.ID, lntypes.Hash{34, 56, 78, 90}, 789456123789,
	)
	require.NoError(t, err)
	err = store.UpdateAccountPaymentStatus(
		ctx, acct1.ID, lntypes.Hash{34, 56, 78, 90},
		lnrpc.Payment_SUCCEEDED,
	)
	require.NoError(t, err)
	err = store.AddAccountInvoice(
		ctx, acct1.ID, lntypes.Hash{12, 34, 56, 78},
	)
	require.NoError(t, err)
	err = store.AddAccountInvoice(
		ctx, acct1.ID, lntypes.Hash{34, 56, 78, 90},
	)
	require.NoError(t, err)

	acct1.CurrentBalance = -500
	acct1.ExpirationDate = now
	acct1.Payments[lntypes.Hash{12, 34, 56, 78}] = &PaymentEntry{
		Status:     lnrpc.Payment_FAILED,
		FullAmount: 123456,
	}
	acct1.Payments[lntypes.Hash{34, 56, 78, 90}] = &PaymentEntry{
		Status:     lnrpc.Payment_SUCCEEDED,
		FullAmount: 789456123789,
	}
	acct1.Invoices[lntypes.Hash{12, 34, 56, 78}] = struct{}{}
	acct1.Invoices[lntypes.Hash{34, 56, 78, 90}] = struct{}{}

	dbAccount, err = store.Account(ctx, acct1.ID)
	require.NoError(t, err)
	assertEqualAccounts(t, acct1, dbAccount)

	// Sleep just a tiny bit to make sure we are never too quick to measure
	// the expiry, even though the time is nanosecond scale and writing to
	// the store and reading again should take at least a couple of
	// microseconds.
	time.Sleep(5 * time.Millisecond)
	require.True(t, acct1.HasExpired())

	// Test listing and deleting accounts.
	accounts, err := store.Accounts(ctx)
	require.NoError(t, err)
	require.Len(t, accounts, 1)

	err = store.RemoveAccount(ctx, acct1.ID)
	require.NoError(t, err)

	accounts, err = store.Accounts(ctx)
	require.NoError(t, err)
	require.Len(t, accounts, 0)

	_, err = store.Account(ctx, acct1.ID)
	require.ErrorIs(t, err, ErrAccNotFound)
}

// assertEqualAccounts asserts that two accounts are equal. This helper function
// is needed because an account contains two time.Time values that cannot be
// compared using reflect.DeepEqual().
func assertEqualAccounts(t *testing.T, expected,
	actual *OffChainBalanceAccount) {

	expectedExpiry := expected.ExpirationDate
	actualExpiry := actual.ExpirationDate
	expectedUpdate := expected.LastUpdate
	actualUpdate := actual.LastUpdate

	expected.ExpirationDate = time.Time{}
	expected.LastUpdate = time.Time{}
	actual.ExpirationDate = time.Time{}
	actual.LastUpdate = time.Time{}

	require.Equal(t, expected, actual)
	require.Equal(t, expectedExpiry.UnixNano(), actualExpiry.UnixNano())
	//	require.Equal(t, expectedUpdate.UnixNano(), actualUpdate.UnixNano())

	// Restore the old values to not influence the tests.
	expected.ExpirationDate = expectedExpiry
	expected.LastUpdate = expectedUpdate
	actual.ExpirationDate = actualExpiry
	actual.LastUpdate = actualUpdate
}

// testLastInvoiceIndexes makes sure the last known invoice indexes can be
// stored and retrieved correctly.
func testLastInvoiceIndexes(t *testing.T, makeDB func(t *testing.T) Store) {
	t.Parallel()
	ctx := context.Background()

	store := makeDB(t)

	_, _, err := store.LastIndexes(ctx)
	require.ErrorIs(t, err, ErrNoInvoiceIndexKnown)

	require.NoError(t, store.StoreLastIndexes(ctx, 7, 99))

	add, settle, err := store.LastIndexes(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 7, add)
	require.EqualValues(t, 99, settle)
}

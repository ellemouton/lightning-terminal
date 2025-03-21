package accounts

import (
	"context"
	"github.com/lightningnetwork/lnd/lnwire"
	"testing"
	"time"

	"github.com/lightningnetwork/lnd/clock"
	"github.com/lightningnetwork/lnd/fn"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/stretchr/testify/require"
)

// TestAccountStore tests that accounts can be stored and retrieved correctly.
func TestAccountStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	clock := clock.NewTestClock(time.Now())
	store := NewTestDB(t, clock)

	// Create an account that does not expire.
	acct1, err := store.NewAccount(ctx, 0, time.Time{}, "foo")
	require.NoError(t, err)
	require.False(t, acct1.HasExpired())

	dbAccount, err := store.Account(ctx, acct1.ID)
	require.NoError(t, err)

	assertEqualAccounts(t, acct1, dbAccount)

	// Make sure we cannot create a second account with the same label.
	_, err = store.NewAccount(ctx, 123, time.Time{}, "foo")
	require.ErrorIs(t, err, ErrLabelAlreadyExists)

	// Make sure we cannot set a label that looks like an account ID.
	_, err = store.NewAccount(ctx, 123, time.Time{}, "0011223344556677")
	require.ErrorContains(t, err, "is not allowed as it can be mistaken")

	now := clock.Now()

	// Update all values of the account that we can modify.
	//
	// Update the balance and expiry.
	err = store.UpdateAccountBalanceAndExpiry(
		ctx, acct1.ID, fn.Some(int64(-500)), fn.Some(now),
	)
	require.NoError(t, err)

	// Add 2 payments.
	_, err = store.UpsertAccountPayment(
		ctx, acct1.ID, lntypes.Hash{12, 34, 56, 78}, 123456,
		lnrpc.Payment_FAILED,
	)
	require.NoError(t, err)

	_, err = store.UpsertAccountPayment(
		ctx, acct1.ID, lntypes.Hash{34, 56, 78, 90}, 789456123789,
		lnrpc.Payment_SUCCEEDED,
	)
	require.NoError(t, err)

	// Add 2 invoices.
	err = store.AddAccountInvoice(
		ctx, acct1.ID, lntypes.Hash{12, 34, 56, 78},
	)
	require.NoError(t, err)
	err = store.AddAccountInvoice(
		ctx, acct1.ID, lntypes.Hash{34, 56, 78, 90},
	)
	require.NoError(t, err)

	// Adjust the account balance by first crediting 10000, and then
	// debiting 5000.
	err = store.CreditAccount(ctx, acct1.ID, lnwire.MilliSatoshi(10000))
	require.NoError(t, err)

	err = store.DebitAccount(ctx, acct1.ID, lnwire.MilliSatoshi(5000))
	require.NoError(t, err)

	// Update the in-memory account so that we can compare it with the
	// account we get from the store.
	acct1.CurrentBalance = -500
	acct1.ExpirationDate = clock.Now()
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
	acct1.CurrentBalance += 10000
	acct1.CurrentBalance -= 5000

	dbAccount, err = store.Account(ctx, acct1.ID)
	require.NoError(t, err)
	assertEqualAccounts(t, acct1, dbAccount)

	// Test that adjusting the balance to exactly 0 should work, while
	// adjusting the balance to below 0 should fail.
	err = store.DebitAccount(
		ctx, acct1.ID, lnwire.MilliSatoshi(acct1.CurrentBalance),
	)
	require.NoError(t, err)

	acct1.CurrentBalance = 0

	dbAccount, err = store.Account(ctx, acct1.ID)
	require.NoError(t, err)
	assertEqualAccounts(t, acct1, dbAccount)

	// Adjusting the value to below 0 should fail.
	err = store.DebitAccount(ctx, acct1.ID, lnwire.MilliSatoshi(1))
	require.ErrorContains(t, err, "balance would be below 0")

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
	require.Equal(t, expectedExpiry.Unix(), actualExpiry.Unix())
	require.Equal(t, expectedUpdate.Unix(), actualUpdate.Unix())

	// Restore the old values to not influence the tests.
	expected.ExpirationDate = expectedExpiry
	expected.LastUpdate = expectedUpdate
	actual.ExpirationDate = actualExpiry
	actual.LastUpdate = actualUpdate
}

// TestAccountUpdateMethods tests that all the Store methods that update an
// account work correctly.
func TestAccountUpdateMethods(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("UpdateAccountBalanceAndExpiry", func(t *testing.T) {
		clock := clock.NewTestClock(time.Now())
		store := NewTestDB(t, clock)

		// Ensure that the function errors out if we try update an
		// account that does not exist.
		err := store.UpdateAccountBalanceAndExpiry(
			ctx, AccountID{}, fn.None[int64](),
			fn.None[time.Time](),
		)
		require.ErrorIs(t, err, ErrAccNotFound)

		acct, err := store.NewAccount(ctx, 0, time.Time{}, "foo")
		require.NoError(t, err)

		assertBalanceAndExpiry := func(balance int64,
			expiry time.Time) {

			dbAcct, err := store.Account(ctx, acct.ID)
			require.NoError(t, err)
			require.EqualValues(t, balance, dbAcct.CurrentBalance)
			require.WithinDuration(
				t, expiry, dbAcct.ExpirationDate, time.Second,
			)
		}

		// Get the account from the store and check to see what its
		// initial balance and expiry fields are set to.
		assertBalanceAndExpiry(0, time.Time{})

		// Now, update just the balance of the account.
		newBalance := int64(123)
		err = store.UpdateAccountBalanceAndExpiry(
			ctx, acct.ID, fn.Some(newBalance), fn.None[time.Time](),
		)
		require.NoError(t, err)
		assertBalanceAndExpiry(newBalance, time.Time{})

		// Now update just the expiry of the account.
		newExpiry := clock.Now().Add(time.Hour)
		err = store.UpdateAccountBalanceAndExpiry(
			ctx, acct.ID, fn.None[int64](), fn.Some(newExpiry),
		)
		require.NoError(t, err)
		assertBalanceAndExpiry(newBalance, newExpiry)

		// Update both the balance and expiry of the account.
		newBalance = 456
		newExpiry = clock.Now().Add(2 * time.Hour)
		err = store.UpdateAccountBalanceAndExpiry(
			ctx, acct.ID, fn.Some(newBalance), fn.Some(newExpiry),
		)
		require.NoError(t, err)
		assertBalanceAndExpiry(newBalance, newExpiry)

		// Finally, test an update that has no net changes to the
		// balance or expiry.
		err = store.UpdateAccountBalanceAndExpiry(
			ctx, acct.ID, fn.None[int64](), fn.None[time.Time](),
		)
		require.NoError(t, err)
		assertBalanceAndExpiry(newBalance, newExpiry)
	})

	t.Run("AddAccountInvoice", func(t *testing.T) {
		store := NewTestDB(t, clock.NewTestClock(time.Now()))

		acct, err := store.NewAccount(ctx, 0, time.Time{}, "foo")
		require.NoError(t, err)

		assertInvoices := func(invoices ...lntypes.Hash) {
			dbAcct, err := store.Account(ctx, acct.ID)
			require.NoError(t, err)

			// First make sure the number of invoices match before
			// de-duping the hashes.
			require.Len(t, dbAcct.Invoices, len(invoices))

			dbInvs := make([]lntypes.Hash, 0, len(dbAcct.Invoices))
			for hash := range dbAcct.Invoices {
				dbInvs = append(dbInvs, hash)
			}

			require.ElementsMatch(t, invoices, dbInvs)
		}

		// The account initially has no invoices.
		assertInvoices()

		// Adding an invoice to an account that doesnt exist yet should
		// error out.
		err = store.AddAccountInvoice(ctx, AccountID{}, lntypes.Hash{})
		require.ErrorIs(t, err, ErrAccNotFound)

		// Add an invoice to the account.
		hash1 := lntypes.Hash{1, 2, 3, 4}
		err = store.AddAccountInvoice(ctx, acct.ID, hash1)
		require.NoError(t, err)

		assertInvoices(hash1)

		// Assert that adding the same invoice again does not change the
		// state.
		err = store.AddAccountInvoice(ctx, acct.ID, hash1)
		require.NoError(t, err)

		assertInvoices(hash1)

		// Now add a second invoice.
		hash2 := lntypes.Hash{5, 6, 7, 8}
		err = store.AddAccountInvoice(ctx, acct.ID, hash2)
		require.NoError(t, err)

		assertInvoices(hash1, hash2)
	})

	t.Run("CreditAccount", func(t *testing.T) {
		store := NewTestDB(t, clock.NewTestClock(time.Now()))

		// Increasing the balance of an account that doesn't exist
		// should error out.
		err := store.CreditAccount(ctx, AccountID{}, 100)
		require.ErrorIs(t, err, ErrAccNotFound)

		acct, err := store.NewAccount(ctx, 123, time.Time{}, "foo")
		require.NoError(t, err)

		assertBalance := func(balance int64) {
			dbAcct, err := store.Account(ctx, acct.ID)
			require.NoError(t, err)
			require.EqualValues(t, balance, dbAcct.CurrentBalance)
		}

		// The account initially has a balance of 123.
		assertBalance(123)

		// Increase the balance by 100 and assert that the new balance
		// is 223.
		err = store.CreditAccount(ctx, acct.ID, 100)
		require.NoError(t, err)

		assertBalance(223)
	})

	t.Run("Upsert and Delete AccountPayment", func(t *testing.T) {
		store := NewTestDB(t, clock.NewTestClock(time.Now()))

		acct, err := store.NewAccount(ctx, 1000, time.Time{}, "foo")
		require.NoError(t, err)

		assertBalanceAndPayments := func(balance int64,
			payments AccountPayments) {

			dbAcct, err := store.Account(ctx, acct.ID)
			require.NoError(t, err)
			require.EqualValues(t, balance, dbAcct.CurrentBalance)

			require.Len(t, dbAcct.Payments, len(payments))
			for hash, payment := range payments {
				dbPayment, ok := dbAcct.Payments[hash]
				require.True(t, ok)
				require.Equal(t, payment, dbPayment)
			}
		}

		// The account initially has a balance of 1000 and no payments.
		assertBalanceAndPayments(1000, nil)

		// Assert that calling the method for a non-existent account
		// errors out.
		_, err = store.UpsertAccountPayment(
			ctx, AccountID{}, lntypes.Hash{}, 0,
			lnrpc.Payment_UNKNOWN,
		)
		require.ErrorIs(t, err, ErrAccNotFound)

		// Add a payment to the account but don't update the balance.
		// We do add a WithErrIfAlreadyPending and
		// WithErrIfAlreadySucceeded option. here just to show that no
		// error is returned since the payment does not exist yet.
		hash1 := lntypes.Hash{1, 2, 3, 4}
		known, err := store.UpsertAccountPayment(
			ctx, acct.ID, hash1, 600, lnrpc.Payment_UNKNOWN,
			WithErrIfAlreadyPending(),
			WithErrIfAlreadySucceeded(),
		)
		require.NoError(t, err)
		require.False(t, known)

		assertBalanceAndPayments(1000, AccountPayments{
			hash1: &PaymentEntry{
				Status:     lnrpc.Payment_UNKNOWN,
				FullAmount: 600,
			},
		})

		// Add a second payment to the account and again don't update
		// the balance.
		hash2 := lntypes.Hash{5, 6, 7, 8}
		known, err = store.UpsertAccountPayment(
			ctx, acct.ID, hash2, 100, lnrpc.Payment_UNKNOWN,
		)
		require.NoError(t, err)
		require.False(t, known)

		assertBalanceAndPayments(1000, AccountPayments{
			hash1: &PaymentEntry{
				Status:     lnrpc.Payment_UNKNOWN,
				FullAmount: 600,
			},
			hash2: &PaymentEntry{
				Status:     lnrpc.Payment_UNKNOWN,
				FullAmount: 100,
			},
		})

		// Now, update the first payment to have a new status and this
		// time, debit the account.
		known, err = store.UpsertAccountPayment(
			ctx, acct.ID, hash1, 600, lnrpc.Payment_SUCCEEDED,
			WithDebitAccount(),
		)
		require.NoError(t, err)
		require.True(t, known)

		// The account should now have a balance of 400 and the first
		// payment should have a status of succeeded.
		assertBalanceAndPayments(400, AccountPayments{
			hash1: &PaymentEntry{
				Status:     lnrpc.Payment_SUCCEEDED,
				FullAmount: 600,
			},
			hash2: &PaymentEntry{
				Status:     lnrpc.Payment_UNKNOWN,
				FullAmount: 100,
			},
		})

		// Calling the same method again with the same payment hash
		// should have no effect by default.
		known, err = store.UpsertAccountPayment(
			ctx, acct.ID, hash1, 600, lnrpc.Payment_SUCCEEDED,
		)
		require.NoError(t, err)
		require.True(t, known)

		assertBalanceAndPayments(400, AccountPayments{
			hash1: &PaymentEntry{
				Status:     lnrpc.Payment_SUCCEEDED,
				FullAmount: 600,
			},
			hash2: &PaymentEntry{
				Status:     lnrpc.Payment_UNKNOWN,
				FullAmount: 100,
			},
		})

		// But, if we use the WithErrIfAlreadyPending option, we should
		// get an error since the payment already exists.
		known, err = store.UpsertAccountPayment(
			ctx, acct.ID, hash1, 600, lnrpc.Payment_SUCCEEDED,
			WithErrIfAlreadyPending(),
		)
		require.ErrorContains(t, err, "is already in flight")
		require.True(t, known)

		// Do the above call again but this time, use the
		// WithErrIfAlreadySucceeded option. This should return the
		// ErrAlreadySucceeded error since the payment has already
		// succeeded.
		known, err = store.UpsertAccountPayment(
			ctx, acct.ID, hash1, 600, lnrpc.Payment_SUCCEEDED,
			WithErrIfAlreadySucceeded(),
		)
		require.ErrorIs(t, err, ErrAlreadySucceeded)
		require.True(t, known)

		// We now call the method again for hash 2 and update its status
		// to SUCCEEDED. This time, we will use the WithPendingAmount
		// option which means that whatever `fullAmount` is passed in
		// should be ignored and the pending amount should be used
		// instead.
		known, err = store.UpsertAccountPayment(
			ctx, acct.ID, hash2, 0, lnrpc.Payment_SUCCEEDED,
			WithPendingAmount(),
		)
		require.NoError(t, err)
		require.True(t, known)

		assertBalanceAndPayments(400, AccountPayments{
			hash1: &PaymentEntry{
				Status:     lnrpc.Payment_SUCCEEDED,
				FullAmount: 600,
			},
			hash2: &PaymentEntry{
				Status:     lnrpc.Payment_SUCCEEDED,
				FullAmount: 100,
			},
		})

		// Delete the first payment and make sure it is removed from the
		// account.
		err = store.DeleteAccountPayment(ctx, acct.ID, hash1)
		require.NoError(t, err)

		assertBalanceAndPayments(400, AccountPayments{
			hash2: &PaymentEntry{
				Status:     lnrpc.Payment_SUCCEEDED,
				FullAmount: 100,
			},
		})

		// Test that deleting a payment that does not exist returns an
		// error.
		err = store.DeleteAccountPayment(ctx, acct.ID, hash1)
		require.ErrorIs(t, err, ErrPaymentNotAssociated)

		// Try once more to insert a payment that is currently unknown
		// but this time add the WithErrIfUnknown option. This should
		// return the ErrPaymentNotAssociated error.
		_, err = store.UpsertAccountPayment(
			ctx, acct.ID, hash1, 600, lnrpc.Payment_SUCCEEDED,
			WithErrIfUnknown(),
		)
		require.ErrorIs(t, err, ErrPaymentNotAssociated)

		// Show that using the two options WithErrIfUnknown and
		// WithPendingAmount together will return the
		// ErrPaymentNotAssociated and will not successfully update
		// the status. We call this for hash1 since it is no longer
		// known. We do this to simulate the behaviour of
		// removePayment.
		_, err = store.UpsertAccountPayment(
			ctx, acct.ID, hash1, 0, lnrpc.Payment_SUCCEEDED,
			WithErrIfUnknown(),
			WithPendingAmount(),
		)
		require.ErrorIs(t, err, ErrPaymentNotAssociated)

		assertBalanceAndPayments(400, AccountPayments{
			hash2: &PaymentEntry{
				Status:     lnrpc.Payment_SUCCEEDED,
				FullAmount: 100,
			},
		})

		// Now insert hash 1 again.
		_, err = store.UpsertAccountPayment(
			ctx, acct.ID, hash1, 600, lnrpc.Payment_IN_FLIGHT,
		)
		require.NoError(t, err)

		assertBalanceAndPayments(400, AccountPayments{
			hash1: &PaymentEntry{
				Status:     lnrpc.Payment_IN_FLIGHT,
				FullAmount: 600,
			},
			hash2: &PaymentEntry{
				Status:     lnrpc.Payment_SUCCEEDED,
				FullAmount: 100,
			},
		})

		// Once again call UpsertAccountPayment with both the
		// WithErrIfUnknown and WithPendingAmount options. This time
		// it should succeed since the payment is now known and so the
		// status should be updated.
		_, err = store.UpsertAccountPayment(
			ctx, acct.ID, hash1, 0, lnrpc.Payment_SUCCEEDED,
			WithErrIfUnknown(),
			WithPendingAmount(),
		)
		require.NoError(t, err)

		assertBalanceAndPayments(400, AccountPayments{
			hash1: &PaymentEntry{
				Status:     lnrpc.Payment_SUCCEEDED,
				FullAmount: 600,
			},
			hash2: &PaymentEntry{
				Status:     lnrpc.Payment_SUCCEEDED,
				FullAmount: 100,
			},
		})
	})
}

// TestLastInvoiceIndexes makes sure the last known invoice indexes can be
// stored and retrieved correctly.
func TestLastInvoiceIndexes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store := NewTestDB(t, clock.NewTestClock(time.Now()))

	_, _, err := store.LastIndexes(ctx)
	require.ErrorIs(t, err, ErrNoInvoiceIndexKnown)

	require.NoError(t, store.StoreLastIndexes(ctx, 7, 99))

	add, settle, err := store.LastIndexes(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 7, add)
	require.EqualValues(t, 99, settle)
}

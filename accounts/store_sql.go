package accounts

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/lightninglabs/lightning-terminal/db"
	"github.com/lightninglabs/lightning-terminal/db/sqlc"
	"github.com/lightningnetwork/lnd/fn"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
)

const (
	addIndexName    = "last_add_index"
	settleIndexName = "last_settle_index"
)

// SQLQueries is a subset of the sqlc.Queries interface that can be used
// to interact with accounts related tables.
type SQLQueries interface {
	AddAccountInvoice(ctx context.Context, arg sqlc.AddAccountInvoiceParams) error
	DeleteAccountPayment(ctx context.Context, arg sqlc.DeleteAccountPaymentParams) error
	GetAccountPayment(ctx context.Context, arg sqlc.GetAccountPaymentParams) (sqlc.AccountPayment, error)
	InsertAccount(ctx context.Context, arg sqlc.InsertAccountParams) (int64, error)
	UpdateAccountBalance(ctx context.Context, arg sqlc.UpdateAccountBalanceParams) error
	UpdateAccountExpiry(ctx context.Context, arg sqlc.UpdateAccountExpiryParams) error
	UpdateAccountLastUpdate(ctx context.Context, arg sqlc.UpdateAccountLastUpdateParams) error
	UpsertAccountPayment(ctx context.Context, arg sqlc.UpsertAccountPaymentParams) error
	GetAccount(ctx context.Context, legacyID []byte) (sqlc.Account, error)
	GetAccountByLabel(ctx context.Context, label sql.NullString) (sqlc.Account, error)
	ListAccountInvoices(ctx context.Context, legacyID []byte) ([]sqlc.AccountInvoice, error)
	ListAccountPayments(ctx context.Context, legacyID []byte) ([]sqlc.AccountPayment, error)
	ListAllAccountIDs(ctx context.Context) ([][]byte, error)
	DeleteAccount(ctx context.Context, legacyID []byte) error
	GetAccountIndex(ctx context.Context, name string) (int64, error)
	SetAccountIndex(ctx context.Context, arg sqlc.SetAccountIndexParams) error
	GetAccountByAliasPrefix(ctx context.Context, legacyID []byte) (sqlc.Account, error)
}

// BatchedSQLQueries is a version of the SQLActionQueries that's capable
// of batched database operations.
type BatchedSQLQueries interface {
	SQLQueries

	db.BatchedTx[SQLQueries]
}

// SQLStore represents a storage backend.
type SQLStore struct {
	db BatchedSQLQueries

	*sql.DB
}

// NewSQLStore creates a new SQLStore instance given an open BatchedSQLQueries
// storage backend.
func NewSQLStore(sqlDB *db.BaseDB) *SQLStore {
	executor := db.NewTransactionExecutor(
		sqlDB, func(tx *sql.Tx) SQLQueries {
			return sqlDB.WithTx(tx)
		},
	)

	return &SQLStore{
		db: executor,
		DB: sqlDB.DB,
	}
}

// NewAccount creates and persists a new OffChainBalanceAccount with the given
// balance and a randomly chosen ID. If the given label is not empty, then it
// must be unique; if it is not, then ErrLabelAlreadyExists is returned.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) NewAccount(ctx context.Context, balance lnwire.MilliSatoshi,
	expirationDate time.Time, label string) (*OffChainBalanceAccount,
	error) {

	// Ensure that if a label is set, it can't be mistaken for a hex
	// encoded account ID to avoid confusion and make it easier for the CLI
	// to distinguish between the two.
	var labelVal sql.NullString
	if len(label) > 0 {
		if _, err := hex.DecodeString(label); err == nil &&
			len(label) == hex.EncodedLen(AccountIDLen) {

			return nil, fmt.Errorf("the label '%s' is not allowed "+
				"as it can be mistaken for an account ID",
				label)
		}

		labelVal = sql.NullString{
			String: label,
			Valid:  true,
		}
	}

	var (
		writeTxOpts db.QueriesTxOptions
		account     *OffChainBalanceAccount
	)
	err := s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		// First, find a unique alias (this is what the ID was in the
		// kvdb implementation of the DB).
		alias, err := uniqueRandomLegacyAccountID(ctx, db)
		if err != nil {
			return err
		}

		if labelVal.Valid {
			_, err = db.GetAccountByLabel(ctx, labelVal)
			if err == nil {
				return ErrLabelAlreadyExists
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}

		_, err = db.InsertAccount(ctx, sqlc.InsertAccountParams{
			Type:               int16(TypeInitialBalance),
			IntialBalanceMsat:  int64(balance),
			CurrentBalanceMsat: int64(balance),
			Expiration:         expirationDate,
			LastUpdated:        time.Now(),
			Label:              labelVal,
			Alias:              alias[:],
		})
		if err != nil {
			return fmt.Errorf("inserting account: %w", err)
		}

		account, err = getAndMarshalAccount(ctx, db, alias[:])
		if err != nil {
			return fmt.Errorf("fetching account: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return account, nil
}

// getAndMarshalAccount retrieves the account with the given alias. If the
// account cannot be found, then ErrAccNotFound is returned.
func getAndMarshalAccount(ctx context.Context, db SQLQueries, alias []byte) (
	*OffChainBalanceAccount, error) {

	dbAcct, err := db.GetAccount(ctx, alias)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAccNotFound
	} else if err != nil {
		return nil, err
	}

	return marshalDBAccount(ctx, db, dbAcct)
}

func marshalDBAccount(ctx context.Context, db SQLQueries,
	dbAcct sqlc.Account) (*OffChainBalanceAccount, error) {

	var alias AccountID
	copy(alias[:], dbAcct.Alias)

	account := &OffChainBalanceAccount{
		ID:             alias,
		Type:           AccountType(dbAcct.Type),
		InitialBalance: lnwire.MilliSatoshi(dbAcct.IntialBalanceMsat),
		CurrentBalance: dbAcct.CurrentBalanceMsat,
		LastUpdate:     dbAcct.LastUpdated,
		ExpirationDate: dbAcct.Expiration,
		Invoices:       make(AccountInvoices),
		Payments:       make(AccountPayments),
		Label:          dbAcct.Label.String,
	}

	invoices, err := db.ListAccountInvoices(ctx, dbAcct.Alias)
	if err != nil {
		return nil, err
	}
	for _, invoice := range invoices {
		var hash lntypes.Hash
		copy(hash[:], invoice.Hash)
		account.Invoices[hash] = struct{}{}
	}

	payments, err := db.ListAccountPayments(ctx, dbAcct.Alias)
	if err != nil {
		return nil, err
	}

	for _, payment := range payments {
		var hash lntypes.Hash
		copy(hash[:], payment.Hash)
		account.Payments[hash] = &PaymentEntry{
			Status:     lnrpc.Payment_PaymentStatus(payment.Status),
			FullAmount: lnwire.MilliSatoshi(payment.FullAmountMsat),
		}
	}

	return account, nil
}

func uniqueRandomLegacyAccountID(ctx context.Context, db SQLQueries) (AccountID,
	error) {

	var (
		newID    AccountID
		numTries = 10
	)
	for numTries > 0 {
		if _, err := rand.Read(newID[:]); err != nil {
			return newID, err
		}

		_, err := db.GetAccount(ctx, newID[:])
		if errors.Is(err, sql.ErrNoRows) {
			// No account found with this new ID, we can use it.
			return newID, nil
		} else if err != nil {
			return AccountID{}, err
		}

		numTries--
	}

	return AccountID{}, fmt.Errorf("couldn't create new account ID")
}

// AddAccountInvoice adds and invoice hash to the account with the given ID.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) AddAccountInvoice(ctx context.Context, id AccountID,
	hash lntypes.Hash) error {

	var (
		writeTxOpts db.QueriesTxOptions
	)
	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		// First check that this invoice does not already exist.
		invoices, err := db.ListAccountInvoices(ctx, id[:])
		if err != nil {
			return err
		}

		for _, invoice := range invoices {
			if bytes.Equal(invoice.Hash, hash[:]) {
				return nil
			}
		}

		err = db.AddAccountInvoice(ctx, sqlc.AddAccountInvoiceParams{
			Alias: id[:],
			Hash:  hash[:],
		})
		if err != nil {
			return err
		}

		return db.UpdateAccountLastUpdate(
			ctx, sqlc.UpdateAccountLastUpdateParams{
				Alias:       id[:],
				LastUpdated: time.Now(),
			},
		)
	})
}

// UpdateAccountBalanceAndExpiry updates the balance and/or expiry of an
// account.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) UpdateAccountBalanceAndExpiry(ctx context.Context,
	id AccountID, newBalance fn.Option[int64],
	newExpiry fn.Option[time.Time]) error {

	var (
		writeTxOpts db.QueriesTxOptions
	)
	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		var err error
		newBalance.WhenSome(func(i int64) {
			err = db.UpdateAccountBalance(
				ctx, sqlc.UpdateAccountBalanceParams{
					Alias:              id[:],
					CurrentBalanceMsat: i,
				},
			)
		})
		if err != nil {
			return err
		}

		newExpiry.WhenSome(func(t time.Time) {
			err = db.UpdateAccountExpiry(
				ctx, sqlc.UpdateAccountExpiryParams{
					Alias:      id[:],
					Expiration: t,
				},
			)
		})
		if err != nil {
			return err
		}

		return db.UpdateAccountLastUpdate(
			ctx, sqlc.UpdateAccountLastUpdateParams{
				Alias:       id[:],
				LastUpdated: time.Now(),
			},
		)
	})
}

func (s *SQLStore) AddAccountPayment(ctx context.Context, id AccountID,
	hash lntypes.Hash, fullAmt lnwire.MilliSatoshi) error {

	var (
		writeTxOpts db.QueriesTxOptions
	)
	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		payment, err := db.GetAccountPayment(
			ctx, sqlc.GetAccountPaymentParams{
				Hash:  hash[:],
				Alias: id[:],
			},
		)
		if err == nil {
			// We do not allow another payment to the same hash if
			// the payment is already in-flight or succeeded. This
			// mitigates a user being able to launch a second
			// RPC-erring payment with the same hash that would
			// remove the payment from being tracked. Note that
			// this prevents launching multipart payments, but
			// allows retrying a payment if it has failed.
			if payment.Status != int16(lnrpc.Payment_FAILED) {
				return fmt.Errorf("payment with hash %s is "+
					"already in flight or succeeded "+
					"(status %v)", hash,
					lnrpc.Payment_PaymentStatus(
						payment.Status,
					))
			}

			// Otherwise, we fall through to correctly update the
			// payment amount, in case we have a zero-amount invoice
			// that is retried.
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		err = db.UpsertAccountPayment(
			ctx, sqlc.UpsertAccountPaymentParams{
				Alias:          id[:],
				Hash:           hash[:],
				Status:         int16(lnrpc.Payment_UNKNOWN),
				FullAmountMsat: int64(fullAmt),
			},
		)
		if err != nil {
			return err
		}

		return db.UpdateAccountLastUpdate(
			ctx, sqlc.UpdateAccountLastUpdateParams{
				Alias:       id[:],
				LastUpdated: time.Now(),
			},
		)
	})
}

func (s *SQLStore) SetAccountPaymentErrored(ctx context.Context, id AccountID, hash lntypes.Hash) error {
	var (
		writeTxOpts db.QueriesTxOptions
	)
	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		_, err := db.GetAccountPayment(ctx, sqlc.GetAccountPaymentParams{
			Alias: id[:],
			Hash:  hash[:],
		})
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("payment with hash %s is not "+
				"associated with this account", hash)
		} else if err != nil {
			return err
		}

		err = db.DeleteAccountPayment(
			ctx, sqlc.DeleteAccountPaymentParams{
				Alias: id[:],
				Hash:  hash[:],
			},
		)
		if err != nil {
			return err
		}

		return db.UpdateAccountLastUpdate(
			ctx, sqlc.UpdateAccountLastUpdateParams{
				Alias:       id[:],
				LastUpdated: time.Now(),
			},
		)
	})
}

// IncreaseAccountBalance increases the balance of the account with the given ID
// by the given amount.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) IncreaseAccountBalance(ctx context.Context, id AccountID,
	amount lnwire.MilliSatoshi) error {

	var (
		writeTxOpts db.QueriesTxOptions
	)
	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		acct, err := db.GetAccount(ctx, id[:])
		if err != nil {
			return err
		}

		newBalance := acct.CurrentBalanceMsat + int64(amount)

		err = db.UpdateAccountBalance(
			ctx, sqlc.UpdateAccountBalanceParams{
				Alias:              id[:],
				CurrentBalanceMsat: newBalance,
			},
		)
		if err != nil {
			return err
		}

		return db.UpdateAccountLastUpdate(
			ctx, sqlc.UpdateAccountLastUpdateParams{
				Alias:       id[:],
				LastUpdated: time.Now(),
			},
		)
	})
}

// Account retrieves an account from the SQL store and un-marshals it. If the
// account cannot be found, then ErrAccNotFound is returned.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) Account(ctx context.Context, id AccountID) (*OffChainBalanceAccount, error) {
	var (
		readTxOpts = db.NewQueryReadTx()
		account    *OffChainBalanceAccount
	)
	err := s.db.ExecTx(ctx, &readTxOpts, func(db SQLQueries) error {
		var err error
		account, err = getAndMarshalAccount(ctx, db, id[:])
		return err
	})

	return account, err
}

// Accounts retrieves all accounts from the SQL store and un-marshals them.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) Accounts(ctx context.Context) ([]*OffChainBalanceAccount, error) {
	var (
		readTxOpts = db.NewQueryReadTx()
		accounts   []*OffChainBalanceAccount
	)
	err := s.db.ExecTx(ctx, &readTxOpts, func(db SQLQueries) error {
		ids, err := db.ListAllAccountIDs(ctx)
		if err != nil {
			return err
		}

		accounts = make([]*OffChainBalanceAccount, len(ids))
		for i, id := range ids {
			account, err := getAndMarshalAccount(ctx, db, id)
			if err != nil {
				return err
			}

			accounts[i] = account
		}

		return nil
	})

	return accounts, err
}

// RemoveAccount finds an account by its ID and removes it from the DB.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) RemoveAccount(ctx context.Context, id AccountID) error {
	var writeTxOpts db.QueriesTxOptions
	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		return db.DeleteAccount(ctx, id[:])
	})
}

// UpsertAccountPayment updates or inserts a payment entry for the given
// account. Various functional options can be passed to modify the behavior of
// the method. The returned boolean is true if the payment was already known
// before the update. This is to be treated as a best-effort indication if an
// error is also returned since the method may error before the boolean can be
// set correctly.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) UpsertAccountPayment(ctx context.Context, id AccountID,
	hash lntypes.Hash, fullAmount lnwire.MilliSatoshi,
	status lnrpc.Payment_PaymentStatus,
	options ...UpsertPaymentOption) (bool, error) {

	opts := newUpsertPaymentOption()
	for _, o := range options {
		o(opts)
	}

	var (
		writeTxOpts db.QueriesTxOptions
		known       bool
	)
	return known, s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		payment, err := db.GetAccountPayment(
			ctx, sqlc.GetAccountPaymentParams{
				Alias: id[:],
				Hash:  hash[:],
			},
		)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		known = err == nil

		if known {
			currStatus := lnrpc.Payment_PaymentStatus(
				payment.Status,
			)
			if opts.errIfAlreadySucceeded &&
				successState(currStatus) {

				return ErrAlreadySucceeded
			}

			// If the errIfAlreadyPending option is set, we return
			// an error if the payment is already in-flight or
			// succeeded.
			if opts.errIfAlreadyPending &&
				currStatus != lnrpc.Payment_FAILED {

				return fmt.Errorf("payment with hash %s is "+
					"already in flight or succeeded "+
					"(status %v)", hash, currStatus)
			}

			if opts.usePendingAmount {
				fullAmount = lnwire.MilliSatoshi(
					payment.FullAmountMsat,
				)
			}
		} else if opts.errIfUnknown {
			return ErrPaymentUnknown
		}

		err = db.UpsertAccountPayment(
			ctx, sqlc.UpsertAccountPaymentParams{
				Alias:          id[:],
				Hash:           hash[:],
				Status:         int16(status),
				FullAmountMsat: int64(fullAmount),
			},
		)
		if err != nil {
			return err
		}

		if opts.debitAccount {
			acct, err := db.GetAccount(ctx, id[:])
			if err != nil {
				return err
			}

			err = db.UpdateAccountBalance(ctx, sqlc.UpdateAccountBalanceParams{
				Alias:              id[:],
				CurrentBalanceMsat: acct.CurrentBalanceMsat - int64(fullAmount),
			})
			if err != nil {
				return err
			}
		}

		return db.UpdateAccountLastUpdate(
			ctx, sqlc.UpdateAccountLastUpdateParams{
				Alias:       id[:],
				LastUpdated: time.Now(),
			},
		)
	})
}

// DeleteAccountPayment removes a payment entry from the account with the given
// ID. It will return an error if the payment is not associated with the
// account.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) DeleteAccountPayment(ctx context.Context, id AccountID,
	hash lntypes.Hash) error {

	var writeTxOpts db.QueriesTxOptions
	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		_, err := db.GetAccountPayment(
			ctx, sqlc.GetAccountPaymentParams{
				Alias: id[:],
				Hash:  hash[:],
			},
		)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("payment with hash %s is not "+
				"associated with this account", hash)
		} else if err != nil {
			return err
		}

		err = db.DeleteAccountPayment(
			ctx, sqlc.DeleteAccountPaymentParams{
				Alias: id[:],
				Hash:  hash[:],
			},
		)
		if err != nil {
			return err
		}

		return db.UpdateAccountLastUpdate(
			ctx, sqlc.UpdateAccountLastUpdateParams{
				Alias:       id[:],
				LastUpdated: time.Now(),
			},
		)
	})
}

// LastIndexes returns the last invoice add and settle index or
// ErrNoInvoiceIndexKnown if no indexes are known yet.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) LastIndexes(ctx context.Context) (uint64, uint64, error) {
	var (
		readTxOpts            = db.NewQueryReadTx()
		addIndex, settleIndex int64
	)
	err := s.db.ExecTx(ctx, &readTxOpts, func(db SQLQueries) error {
		var err error
		addIndex, err = db.GetAccountIndex(ctx, addIndexName)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoInvoiceIndexKnown
		} else if err != nil {
			return err
		}

		settleIndex, err = db.GetAccountIndex(ctx, settleIndexName)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoInvoiceIndexKnown
		}

		return err
	})

	return uint64(addIndex), uint64(settleIndex), err
}

// StoreLastIndexes stores the last invoice add and settle index.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) StoreLastIndexes(ctx context.Context, addIndex, settleIndex uint64) error {
	var writeTxOpts db.QueriesTxOptions
	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		err := db.SetAccountIndex(ctx, sqlc.SetAccountIndexParams{
			Name:  addIndexName,
			Value: int64(addIndex),
		})
		if err != nil {
			return err
		}

		return db.SetAccountIndex(ctx, sqlc.SetAccountIndexParams{
			Name:  settleIndexName,
			Value: int64(settleIndex),
		})
	})
}

// Close closes the underlying store.
//
// NOTE: This is part of the Store interface.
func (s *SQLStore) Close() error {
	return s.DB.Close()
}

var _ Store = (*SQLStore)(nil)

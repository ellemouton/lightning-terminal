package accounts

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/lightninglabs/lightning-terminal/db"
	"github.com/lightninglabs/lightning-terminal/db/sqlc"
	"github.com/lightninglabs/taproot-assets/fn"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
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
	UpdateKVStoreRecord(ctx context.Context, arg sqlc.UpdateKVStoreRecordParams) error
	UpsertAccountPayment(ctx context.Context, arg sqlc.UpsertAccountPaymentParams) error
	GetAccount(ctx context.Context, legacyID []byte) (sqlc.Account, error)
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
}

// NewSQLStore creates a new SQLStore instance given an open BatchedSQLQueries
// storage backend.
func NewSQLStore(db BatchedSQLQueries) *SQLStore {
	return &SQLStore{
		db: db,
	}
}

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
		// First, fine a unique legacy ID.
		legacyID, err := uniqueRandomLegacyAccountID(ctx, db)
		if err != nil {
			return err
		}

		_, err = db.InsertAccount(ctx, sqlc.InsertAccountParams{
			Type:               int16(TypeInitialBalance),
			IntialBalanceMsat:  int64(balance),
			CurrentBalanceMsat: int64(balance),
			Expiration:         expirationDate,
			LastUpdated:        time.Now(),
			Label:              labelVal,
			LegacyID:           legacyID[:],
		})
		if err != nil {
			return err
		}

		account, err = getAndMarshalAccount(ctx, db, legacyID[:])

		return err
	}, func() {})
	if err != nil {
		mappedSQLErr := db.MapSQLError(err)
		var uniqueConstraintErr *db.ErrSQLUniqueConstraintViolation
		if errors.As(mappedSQLErr, &uniqueConstraintErr) {
			// Add context to unique constraint errors.
			return nil, ErrLabelAlreadyExists
		}

		return nil, err
	}

	return account, nil
}

func getAndMarshalAccount(ctx context.Context, db SQLQueries, id []byte) (
	*OffChainBalanceAccount, error) {

	dbAcct, err := db.GetAccount(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAccNotFound
	} else if err != nil {
		return nil, err
	}

	return marshalDBAccount(ctx, db, dbAcct)
}

func marshalDBAccount(ctx context.Context, db SQLQueries,
	dbAcct sqlc.Account) (*OffChainBalanceAccount, error) {

	var legacyID AccountID
	copy(legacyID[:], dbAcct.LegacyID)

	account := &OffChainBalanceAccount{
		ID:             legacyID,
		Type:           AccountType(dbAcct.Type),
		InitialBalance: lnwire.MilliSatoshi(dbAcct.IntialBalanceMsat),
		CurrentBalance: dbAcct.CurrentBalanceMsat,
		LastUpdate:     dbAcct.LastUpdated,
		ExpirationDate: dbAcct.Expiration,
		Invoices:       make(AccountInvoices),
		Payments:       make(AccountPayments),
		Label:          dbAcct.Label.String,
	}

	invoices, err := db.ListAccountInvoices(ctx, dbAcct.LegacyID)
	if err != nil {
		return nil, err
	}
	for _, invoice := range invoices {
		var hash lntypes.Hash
		copy(hash[:], invoice.Hash)
		account.Invoices[hash] = struct{}{}
	}

	payments, err := db.ListAccountPayments(ctx, dbAcct.LegacyID)
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

	return newID, fmt.Errorf("couldn't create new account ID")
}

func (s *SQLStore) AddAccountInvoice(ctx context.Context, id AccountID, hash lntypes.Hash) error {
	var (
		writeTxOpts db.QueriesTxOptions
	)
	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		err := db.AddAccountInvoice(ctx, sqlc.AddAccountInvoiceParams{
			LegacyID: id[:],
			Hash:     hash[:],
		})
		if err != nil {
			return err
		}

		return db.UpdateAccountLastUpdate(
			ctx, sqlc.UpdateAccountLastUpdateParams{
				LegacyID:    id[:],
				LastUpdated: time.Now(),
			},
		)
	}, func() {})
}

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
					LegacyID:           id[:],
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
					LegacyID:   id[:],
					Expiration: t,
				},
			)
		})
		if err != nil {
			return err
		}

		return db.UpdateAccountLastUpdate(
			ctx, sqlc.UpdateAccountLastUpdateParams{
				LegacyID:    id[:],
				LastUpdated: time.Now(),
			},
		)
	}, func() {})
}

func (s *SQLStore) AddAccountPayment(ctx context.Context, id AccountID,
	hash lntypes.Hash, fullAmt lnwire.MilliSatoshi) error {

	var (
		writeTxOpts db.QueriesTxOptions
	)
	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		payment, err := db.GetAccountPayment(
			ctx, sqlc.GetAccountPaymentParams{
				Hash:     hash[:],
				LegacyID: id[:],
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
				LegacyID:       id[:],
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
				LegacyID:    id[:],
				LastUpdated: time.Now(),
			},
		)
	}, func() {})
}

func (s *SQLStore) SetAccountPaymentErrored(ctx context.Context, id AccountID, hash lntypes.Hash) error {
	var (
		writeTxOpts db.QueriesTxOptions
	)
	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {

		_, err := db.GetAccountPayment(ctx, sqlc.GetAccountPaymentParams{
			LegacyID: id[:],
			Hash:     hash[:],
		})
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("payment with hash %s is not "+
				"associated with this account", hash)
		} else if err != nil {
			return err
		}

		err = db.DeleteAccountPayment(
			ctx, sqlc.DeleteAccountPaymentParams{
				LegacyID: id[:],
				Hash:     hash[:],
			},
		)
		if err != nil {
			return err
		}

		return db.UpdateAccountLastUpdate(
			ctx, sqlc.UpdateAccountLastUpdateParams{
				LegacyID:    id[:],
				LastUpdated: time.Now(),
			},
		)
	}, func() {})
}

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
				LegacyID:           id[:],
				CurrentBalanceMsat: newBalance,
			},
		)
		if err != nil {
			return err
		}

		return db.UpdateAccountLastUpdate(
			ctx, sqlc.UpdateAccountLastUpdateParams{
				LegacyID:    id[:],
				LastUpdated: time.Now(),
			},
		)
	}, func() {})
}

func (s *SQLStore) UpdateAccountPaymentStatus(ctx context.Context, id AccountID,
	hash lntypes.Hash, status lnrpc.Payment_PaymentStatus) error {

	var (
		writeTxOpts db.QueriesTxOptions
	)

	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		// Have we associated the payment with the account already?
		payment, err := db.GetAccountPayment(
			ctx, sqlc.GetAccountPaymentParams{
				LegacyID: id[:],
				Hash:     hash[:],
			},
		)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}

		// If we did, let's set the status correctly in the DB now.
		err = db.UpsertAccountPayment(
			ctx, sqlc.UpsertAccountPaymentParams{
				LegacyID:       id[:],
				Hash:           hash[:],
				Status:         int16(status),
				FullAmountMsat: payment.FullAmountMsat,
			},
		)
		if err != nil {
			return err
		}

		return db.UpdateAccountLastUpdate(
			ctx, sqlc.UpdateAccountLastUpdateParams{
				LegacyID:    id[:],
				LastUpdated: time.Now(),
			},
		)
	}, func() {})
}

func (s *SQLStore) UpdateAccountPaymentSuccess(ctx context.Context,
	id AccountID, hash lntypes.Hash, fullAmount lnwire.MilliSatoshi) error {

	var (
		writeTxOpts db.QueriesTxOptions
	)

	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		acct, err := db.GetAccount(ctx, id[:])
		if err != nil {
			return err
		}

		err = db.UpdateAccountBalance(ctx, sqlc.UpdateAccountBalanceParams{
			LegacyID:           id[:],
			CurrentBalanceMsat: acct.CurrentBalanceMsat - int64(fullAmount),
		})
		if err != nil {
			return err
		}

		err = db.UpsertAccountPayment(
			ctx, sqlc.UpsertAccountPaymentParams{
				LegacyID:       id[:],
				Hash:           hash[:],
				Status:         int16(lnrpc.Payment_SUCCEEDED),
				FullAmountMsat: int64(fullAmount),
			},
		)
		if err != nil {
			return err
		}

		return db.UpdateAccountLastUpdate(
			ctx, sqlc.UpdateAccountLastUpdateParams{
				LegacyID:    id[:],
				LastUpdated: time.Now(),
			},
		)
	}, func() {})
}

func (s *SQLStore) UpsertAccountPayment(ctx context.Context, id AccountID,
	hash lntypes.Hash, fullAmt lnwire.MilliSatoshi) (bool, error) {

	var (
		writeTxOpts     db.QueriesTxOptions
		existingPayment bool
	)

	err := s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		payment, err := db.GetAccountPayment(
			ctx, sqlc.GetAccountPaymentParams{
				LegacyID: id[:],
				Hash:     hash[:],
			},
		)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		existingPayment = err == nil
		if existingPayment &&
			successState(lnrpc.Payment_PaymentStatus(payment.Status)) {

			return ErrPaymentAlreadySucceeded
		}
		// There is a case where the passed in fullAmt is zero but the
		// pending amount is not. In that case, we should not overwrite
		// the pending amount.
		if err == nil && fullAmt == 0 {
			fullAmt = lnwire.MilliSatoshi(payment.FullAmountMsat)
		}

		err = db.UpsertAccountPayment(
			ctx, sqlc.UpsertAccountPaymentParams{
				LegacyID:       id[:],
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
				LegacyID:    id[:],
				LastUpdated: time.Now(),
			},
		)
	}, func() {})

	return existingPayment, err
}

func (s *SQLStore) Account(ctx context.Context, id AccountID) (*OffChainBalanceAccount, error) {
	var (
		readTxOpts = db.NewQueryReadTx()
		account    *OffChainBalanceAccount
	)
	err := s.db.ExecTx(ctx, &readTxOpts, func(db SQLQueries) error {
		var err error
		account, err = getAndMarshalAccount(ctx, db, id[:])
		return err
	}, func() {})

	return account, err
}

func (s *SQLStore) GetAccountByIDPrefix(ctx context.Context, prefix [4]byte) (
	*OffChainBalanceAccount, error) {

	var (
		readTxOpts = db.NewQueryReadTx()
		account    *OffChainBalanceAccount
	)
	err := s.db.ExecTx(ctx, &readTxOpts, func(db SQLQueries) error {
		dbAcct, err := db.GetAccountByAliasPrefix(ctx, prefix[:])
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAccNotFound
		} else if err != nil {
			return err
		}

		account, err = marshalDBAccount(ctx, db, dbAcct)

		return err
	}, func() {})

	return account, err
}

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
	}, func() {})

	return accounts, err
}

func (s *SQLStore) RemoveAccount(ctx context.Context, id AccountID) error {
	var writeTxOpts db.QueriesTxOptions
	return s.db.ExecTx(ctx, &writeTxOpts, func(db SQLQueries) error {
		return db.DeleteAccount(ctx, id[:])
	}, func() {})
}

const (
	addIndexName    = "add_index"
	settleIndexName = "settle_index"
)

// LastIndexes returns the last invoice add and settle index or
// ErrNoInvoiceIndexKnown if no indexes are known yet.
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

	}, func() {})

	return uint64(addIndex), uint64(settleIndex), err
}

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
	}, func() {})
}

var _ Store = (*SQLStore)(nil)

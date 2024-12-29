package accounts

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/btcsuite/btcwallet/walletdb"
	"github.com/lightninglabs/taproot-assets/fn"
	"github.com/lightningnetwork/lnd/kvdb"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
	"go.etcd.io/bbolt"
)

var ErrPaymentAlreadySucceeded = fmt.Errorf("payment already succeeded")

const (
	// DBFilename is the filename within the data directory which contains
	// the macaroon stores.
	DBFilename = "accounts.db"

	// dbPathPermission is the default permission the account database
	// directory is created with (if it does not exist).
	dbPathPermission = 0700

	// DefaultAccountDBTimeout is the default maximum time we wait for the
	// account bbolt database to be opened. If the database is already
	// opened by another process, the unique lock cannot be obtained. With
	// the timeout we error out after the given time instead of just
	// blocking for forever.
	DefaultAccountDBTimeout = 5 * time.Second
)

var (
	// accountBucketName is the name of the bucket where all accounting
	// based balances are stored.
	accountBucketName = []byte("accounts")

	// lastAddIndexKey is the name of the key under which we store the last
	// known invoice add index.
	lastAddIndexKey = []byte("last-add-index")

	// lastSettleIndexKey is the name of the key under which we store the
	// last known invoice settle index.
	lastSettleIndexKey = []byte("last-settle-index")

	// byteOrder is the binary byte order we use to encode integers.
	byteOrder = binary.BigEndian

	// zeroID is an empty account ID.
	zeroID = AccountID{}
)

// BoltStore wraps the bolt DB that stores all accounts and their balances.
type BoltStore struct {
	db kvdb.Backend
}

// NewBoltStore creates a BoltStore instance and the corresponding bucket in the
// bolt DB if it does not exist yet.
func NewBoltStore(dir, fileName string) (*BoltStore, error) {
	// Ensure that the path to the directory exists.
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, dbPathPermission); err != nil {
			return nil, err
		}
	}

	// Open the database that we'll use to store the primary macaroon key,
	// and all generated macaroons+caveats.
	db, err := kvdb.GetBoltBackend(&kvdb.BoltBackendConfig{
		DBPath:     dir,
		DBFileName: fileName,
		DBTimeout:  DefaultAccountDBTimeout,
	})
	if err == bbolt.ErrTimeout {
		return nil, fmt.Errorf("error while trying to open %s/%s: "+
			"timed out after %v when trying to obtain exclusive "+
			"lock", dir, fileName, DefaultAccountDBTimeout)
	}
	if err != nil {
		return nil, err
	}

	// If the store's bucket doesn't exist, create it.
	err = db.Update(func(tx kvdb.RwTx) error {
		_, err := tx.CreateTopLevelBucket(accountBucketName)
		return err
	}, func() {})
	if err != nil {
		return nil, err
	}

	// Return the DB wrapped in a BoltStore object.
	return &BoltStore{db: db}, nil
}

func (s *BoltStore) Close() error {
	return s.db.Close()
}

// NewAccount creates a new OffChainBalanceAccount with the given balance and a
// randomly chosen ID.
func (s *BoltStore) NewAccount(ctx context.Context, balance lnwire.MilliSatoshi,
	expirationDate time.Time, label string) (*OffChainBalanceAccount,
	error) {

	// If a label is set, it must be unique, as we use it to identify the
	// account in some of the RPCs. It also can't be mistaken for a hex
	// encoded account ID to avoid confusion and make it easier for the CLI
	// to distinguish between the two.
	if len(label) > 0 {
		if _, err := hex.DecodeString(label); err == nil &&
			len(label) == hex.EncodedLen(AccountIDLen) {

			return nil, fmt.Errorf("the label '%s' is not allowed "+
				"as it can be mistaken for an account ID",
				label)
		}

		accounts, err := s.Accounts(ctx)
		if err != nil {
			return nil, fmt.Errorf("error checking label "+
				"uniqueness: %w", err)
		}
		for _, account := range accounts {
			if account.Label == label {
				return nil, fmt.Errorf("an account with the "+
					"label '%s' already exists: %w", label,
					ErrLabelAlreadyExists)
			}
		}
	}

	// First, create a new instance of an account. Currently, only the type
	// TypeInitialBalance is supported.
	account := &OffChainBalanceAccount{
		Type:           TypeInitialBalance,
		InitialBalance: balance,
		CurrentBalance: int64(balance),
		ExpirationDate: expirationDate,
		LastUpdate:     time.Now(),
		Invoices:       make(AccountInvoices),
		Payments:       make(AccountPayments),
		Label:          label,
	}

	// Try storing the account in the account database, so we can keep track
	// of its balance.
	err := s.db.Update(func(tx walletdb.ReadWriteTx) error {
		bucket := tx.ReadWriteBucket(accountBucketName)
		if bucket == nil {
			return ErrAccountBucketNotFound
		}

		id, err := uniqueRandomAccountID(bucket)
		if err != nil {
			return fmt.Errorf("error creating random account ID: "+
				"%w", err)
		}

		account.ID = id
		return storeAccount(bucket, account)
	}, func() {
		account.ID = zeroID
	})
	if err != nil {
		return nil, err
	}

	return account, nil
}

func (s *BoltStore) AddAccountInvoice(_ context.Context, id AccountID,
	hash lntypes.Hash) error {

	return s.db.Update(func(tx kvdb.RwTx) error {
		bucket := tx.ReadWriteBucket(accountBucketName)
		if bucket == nil {
			return ErrAccountBucketNotFound
		}

		account, err := getAccount(bucket, id)
		if err != nil {
			return err
		}

		account.LastUpdate = time.Now()
		account.Invoices[hash] = struct{}{}

		return storeAccount(bucket, account)
	}, func() {})
}

func (s *BoltStore) UpdateAccountBalanceAndExpiry(_ context.Context,
	id AccountID, newBalance fn.Option[int64],
	newExpiry fn.Option[time.Time]) error {

	return s.db.Update(func(tx kvdb.RwTx) error {
		bucket := tx.ReadWriteBucket(accountBucketName)
		if bucket == nil {
			return ErrAccountBucketNotFound
		}

		account, err := getAccount(bucket, id)
		if err != nil {
			return err
		}

		newBalance.WhenSome(func(balance int64) {
			account.CurrentBalance = balance
		})
		newExpiry.WhenSome(func(expiry time.Time) {
			account.ExpirationDate = expiry
		})

		account.LastUpdate = time.Now()

		return storeAccount(bucket, account)
	}, func() {})
}

func (s *BoltStore) SetAccountPaymentErrored(_ context.Context, id AccountID,
	hash lntypes.Hash) error {

	return s.db.Update(func(tx kvdb.RwTx) error {
		bucket := tx.ReadWriteBucket(accountBucketName)
		if bucket == nil {
			return ErrAccountBucketNotFound
		}

		account, err := getAccount(bucket, id)
		if err != nil {
			return err
		}

		// Check that this payment is actually associated with this
		// account.
		_, ok := account.Payments[hash]
		if !ok {
			return fmt.Errorf("payment with hash %s is not "+
				"associated with this account", hash)
		}

		// Delete the payment and update the persisted account.
		delete(account.Payments, hash)
		account.LastUpdate = time.Now()

		return storeAccount(bucket, account)
	}, func() {})
}

func (s *BoltStore) AddAccountPayment(_ context.Context, id AccountID,
	hash lntypes.Hash, fullAmt lnwire.MilliSatoshi) error {

	return s.db.Update(func(tx kvdb.RwTx) error {
		bucket := tx.ReadWriteBucket(accountBucketName)
		if bucket == nil {
			return ErrAccountBucketNotFound
		}

		account, err := getAccount(bucket, id)
		if err != nil {
			return err
		}

		// Check if this payment is associated with the account already.
		entry, ok := account.Payments[hash]
		if ok {
			// We do not allow another payment to the same hash if
			// the payment is already in-flight or succeeded. This
			// mitigates a user being able to launch a second
			// RPC-erring payment with the same hash that would
			// remove the payment from being tracked. Note that
			// this prevents launching multipart payments, but
			// allows retrying a payment if it has failed.
			if entry.Status != lnrpc.Payment_FAILED {
				return fmt.Errorf("payment with hash %s is "+
					"already in flight or succeeded "+
					"(status %v)", hash,
					entry.Status)
			}

			// Otherwise, we fall through to correctly update the
			// payment amount, in case we have a zero-amount invoice
			// that is retried.
		}

		// Associate the payment with the account and store it.
		account.Payments[hash] = &PaymentEntry{
			Status:     lnrpc.Payment_UNKNOWN,
			FullAmount: fullAmt,
		}
		account.LastUpdate = time.Now()

		return storeAccount(bucket, account)
	}, func() {})
}

func (s *BoltStore) UpsertAccountPayment(_ context.Context, id AccountID,
	hash lntypes.Hash, fullAmt lnwire.MilliSatoshi) (bool, error) {

	var existingPayment bool
	err := s.db.Update(func(tx kvdb.RwTx) error {
		bucket := tx.ReadWriteBucket(accountBucketName)
		if bucket == nil {
			return ErrAccountBucketNotFound
		}

		account, err := getAccount(bucket, id)
		if err != nil {
			return err
		}

		// If the account already stored a terminal state, we also don't
		// need to track the payment again.
		var entry *PaymentEntry
		entry, existingPayment = account.Payments[hash]
		if existingPayment && successState(entry.Status) {
			return ErrPaymentAlreadySucceeded
		}

		// There is a case where the passed in fullAmt is zero but the
		// pending amount is not. In that case, we should not overwrite
		// the pending amount.
		if fullAmt == 0 {
			fullAmt = entry.FullAmount
		}

		account.Payments[hash] = &PaymentEntry{
			Status:     lnrpc.Payment_UNKNOWN,
			FullAmount: fullAmt,
		}
		account.LastUpdate = time.Now()

		return storeAccount(bucket, account)
	}, func() {})

	return existingPayment, err
}

func (s *BoltStore) UpdateAccountPaymentStatus(_ context.Context, id AccountID,
	hash lntypes.Hash, status lnrpc.Payment_PaymentStatus) error {

	return s.db.Update(func(tx kvdb.RwTx) error {
		bucket := tx.ReadWriteBucket(accountBucketName)
		if bucket == nil {
			return ErrAccountBucketNotFound
		}

		account, err := getAccount(bucket, id)
		if err != nil {
			return err
		}

		// Have we associated the payment with the account already?
		_, ok := account.Payments[hash]
		if !ok {
			return nil
		}

		// If we did, let's set the status correctly in the DB now.
		account.Payments[hash].Status = status
		account.LastUpdate = time.Now()

		return storeAccount(bucket, account)
	}, func() {})
}

func (s *BoltStore) UpdateAccountPaymentSuccess(_ context.Context, id AccountID,
	hash lntypes.Hash, fullAmount lnwire.MilliSatoshi) error {

	return s.db.Update(func(tx kvdb.RwTx) error {
		bucket := tx.ReadWriteBucket(accountBucketName)
		if bucket == nil {
			return ErrAccountBucketNotFound
		}

		account, err := getAccount(bucket, id)
		if err != nil {
			return err
		}

		// Update the account and store it in the database.
		account.CurrentBalance -= int64(fullAmount)
		account.Payments[hash] = &PaymentEntry{
			Status:     lnrpc.Payment_SUCCEEDED,
			FullAmount: fullAmount,
		}
		account.LastUpdate = time.Now()

		return storeAccount(bucket, account)
	}, func() {})
}

func (s *BoltStore) IncreaseAccountBalance(_ context.Context, id AccountID,
	amount lnwire.MilliSatoshi) error {

	return s.db.Update(func(tx kvdb.RwTx) error {
		bucket := tx.ReadWriteBucket(accountBucketName)
		if bucket == nil {
			return ErrAccountBucketNotFound
		}

		account, err := getAccount(bucket, id)
		if err != nil {
			return err
		}

		account.CurrentBalance += int64(amount)
		account.LastUpdate = time.Now()

		return storeAccount(bucket, account)
	}, func() {})
}

func getAccount(accountBucket kvdb.RwBucket, id AccountID) (
	*OffChainBalanceAccount, error) {

	accountBinary := accountBucket.Get(id[:])
	if len(accountBinary) == 0 {
		return nil, ErrAccNotFound
	}

	return deserializeAccount(accountBinary)
}

// storeAccount serializes and writes the given account to the given account
// bucket.
func storeAccount(accountBucket kvdb.RwBucket,
	account *OffChainBalanceAccount) error {

	accountBinary, err := serializeAccount(account)
	if err != nil {
		return err
	}

	return accountBucket.Put(account.ID[:], accountBinary)
}

// uniqueRandomAccountID generates a new random ID and makes sure it does not
// yet exist in the DB.
func uniqueRandomAccountID(accountBucket kvdb.RBucket) (AccountID, error) {
	var (
		newID    AccountID
		numTries = 10
	)
	for numTries > 0 {
		if _, err := rand.Read(newID[:]); err != nil {
			return newID, err
		}

		accountBytes := accountBucket.Get(newID[:])
		if accountBytes == nil {
			// No account found with this new ID, we can use it.
			return newID, nil
		}

		numTries--
	}

	return newID, fmt.Errorf("couldn't create new account ID")
}

// Account retrieves an account from the bolt DB and un-marshals it. If the
// account cannot be found, then ErrAccNotFound is returned.
func (s *BoltStore) Account(_ context.Context, id AccountID) (
	*OffChainBalanceAccount, error) {

	// Try looking up and reading the account by its ID from the local
	// bolt DB.
	var accountBinary []byte
	err := s.db.View(func(tx kvdb.RTx) error {
		bucket := tx.ReadBucket(accountBucketName)
		if bucket == nil {
			return ErrAccountBucketNotFound
		}

		accountBinary = bucket.Get(id[:])
		if len(accountBinary) == 0 {
			return ErrAccNotFound
		}

		return nil
	}, func() {
		accountBinary = nil
	})
	if err != nil {
		return nil, err
	}

	// Now try to deserialize the account back from the binary format it was
	// stored in.
	account, err := deserializeAccount(accountBinary)
	if err != nil {
		return nil, err
	}

	return account, nil
}

// Accounts retrieves all accounts from the bolt DB and un-marshals them.
func (s *BoltStore) Accounts(_ context.Context) ([]*OffChainBalanceAccount,
	error) {

	var accounts []*OffChainBalanceAccount
	err := s.db.View(func(tx kvdb.RTx) error {
		// This function will be called in the ForEach and receive
		// the key and value of each account in the DB. The key, which
		// is also the ID is not used because it is also marshaled into
		// the value.
		readFn := func(k, v []byte) error {
			// Skip the two special purpose keys.
			if bytes.Equal(k, lastAddIndexKey) ||
				bytes.Equal(k, lastSettleIndexKey) {

				return nil
			}

			// There should be no sub-buckets.
			if v == nil {
				return fmt.Errorf("invalid bucket structure")
			}

			account, err := deserializeAccount(v)
			if err != nil {
				return err
			}

			accounts = append(accounts, account)
			return nil
		}

		// We know the bucket should exist since it's created when
		// the account storage is initialized.
		return tx.ReadBucket(accountBucketName).ForEach(readFn)
	}, func() {
		accounts = nil
	})
	if err != nil {
		return nil, err
	}

	return accounts, nil
}

// RemoveAccount finds an account by its ID and removes it from the DB.
func (s *BoltStore) RemoveAccount(_ context.Context, id AccountID) error {
	return s.db.Update(func(tx kvdb.RwTx) error {
		bucket := tx.ReadWriteBucket(accountBucketName)
		if bucket == nil {
			return ErrAccountBucketNotFound
		}

		account := bucket.Get(id[:])
		if len(account) == 0 {
			return ErrAccNotFound
		}

		return bucket.Delete(id[:])
	}, func() {})
}

// LastIndexes returns the last invoice add and settle index or
// ErrNoInvoiceIndexKnown if no indexes are known yet.
func (s *BoltStore) LastIndexes(_ context.Context) (uint64, uint64, error) {
	var (
		addValue, settleValue []byte
	)
	err := s.db.View(func(tx kvdb.RTx) error {
		bucket := tx.ReadBucket(accountBucketName)
		if bucket == nil {
			return ErrAccountBucketNotFound
		}

		addValue = bucket.Get(lastAddIndexKey)
		if len(addValue) == 0 {
			return ErrNoInvoiceIndexKnown
		}

		settleValue = bucket.Get(lastSettleIndexKey)
		if len(settleValue) == 0 {
			return ErrNoInvoiceIndexKnown
		}

		return nil
	}, func() {
		addValue, settleValue = nil, nil
	})
	if err != nil {
		return 0, 0, err
	}

	return byteOrder.Uint64(addValue), byteOrder.Uint64(settleValue), nil
}

// StoreLastIndexes stores the last invoice add and settle index.
func (s *BoltStore) StoreLastIndexes(_ context.Context, addIndex,
	settleIndex uint64) error {

	addValue := make([]byte, 8)
	settleValue := make([]byte, 8)
	byteOrder.PutUint64(addValue, addIndex)
	byteOrder.PutUint64(settleValue, settleIndex)

	return s.db.Update(func(tx kvdb.RwTx) error {
		bucket := tx.ReadWriteBucket(accountBucketName)
		if bucket == nil {
			return ErrAccountBucketNotFound
		}

		if err := bucket.Put(lastAddIndexKey, addValue); err != nil {
			return err
		}

		return bucket.Put(lastSettleIndexKey, settleValue)
	}, func() {})
}

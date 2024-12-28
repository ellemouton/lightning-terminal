package accounts

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightninglabs/lndclient"
	"github.com/lightninglabs/taproot-assets/fn"
	"github.com/lightningnetwork/lnd/channeldb"
	invpkg "github.com/lightningnetwork/lnd/invoices"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
)

// Config holds the configuration options for the accounts service.
type Config struct {
	// Disable will disable the accounts service if set.
	Disable bool `long:"disable" description:"disable the accounts service"`
}

// trackedPayment is a struct that holds all information that identifies a
// payment that we are tracking in the service.
type trackedPayment struct {
	// accountID is the ID of the account the payment was associated with.
	accountID AccountID

	// hash is the payment hash of the payment.
	hash lntypes.Hash

	// fullAmount is the total in-flight amount of the payment which
	// includes the payment amount and the estimated routing fee. The
	// routing fee is estimated based on the fee limit set when sending the
	// payment and might be higher than the actual routing fee.
	fullAmount lnwire.MilliSatoshi

	// cancel is the context cancel function that can be called to abort the
	// TrackPayment RPC stream.
	cancel fn.Option[context.CancelFunc]

	errChan chan error
}

// InterceptorService is an account storage and interceptor for accounting based
// macaroon balances and utility methods to manage accounts.
type InterceptorService struct {
	// mu is the read/write mutex that guards all fields that can be
	// accessed by multiple goroutines at the same time, such as the store
	// or pending payments.
	mu sync.RWMutex

	store Store

	routerClient lndclient.RouterClient

	requestMtx sync.Mutex
	checkers   *AccountChecker

	currentAddIndex    uint64
	currentSettleIndex uint64

	invoiceToAccount map[lntypes.Hash]AccountID
	pendingPayments  map[lntypes.Hash]*trackedPayment

	trackPaymentReqs chan *trackedPayment

	*requestValuesStore

	mainErrCallback func(error)
	wg              sync.WaitGroup
	quit            chan struct{}
	cancel          fn.Option[context.CancelFunc]

	isEnabled bool
}

// NewService returns a service backed by the macaroon Bolt DB stored in the
// passed-in directory.
func NewService(store Store, errCallback func(error)) (*InterceptorService,
	error) {

	return &InterceptorService{
		store:              store,
		invoiceToAccount:   make(map[lntypes.Hash]AccountID),
		pendingPayments:    make(map[lntypes.Hash]*trackedPayment),
		trackPaymentReqs:   make(chan *trackedPayment),
		requestValuesStore: newRequestValuesStore(),
		mainErrCallback:    errCallback,
		quit:               make(chan struct{}),
		isEnabled:          false,
	}, nil
}

// Start starts the account service and its interceptor capability.
func (s *InterceptorService) Start(ctx context.Context,
	lightningClient lndclient.LightningClient,
	routerClient lndclient.RouterClient, params *chaincfg.Params) error {

	ctx, cancel := context.WithCancel(ctx)
	s.cancel = fn.Some(cancel)

	s.routerClient = routerClient
	s.checkers = NewAccountChecker(s, params)

	s.isEnabled = true

	s.wg.Add(1)
	go s.trackForever(ctx)

	// Let's first fill our cache that maps invoices to accounts, which
	// allows us to credit an account easily once an invoice is settled. We
	// also track payments that aren't in a final state yet.
	existingAccounts, err := s.store.Accounts(ctx)
	if err != nil {
		s.disable()
		return fmt.Errorf("error querying existing accounts: %w", err)
	}
	for _, acct := range existingAccounts {
		acct := acct
		for invoice := range acct.Invoices {
			invoice := invoice
			s.invoiceToAccount[invoice] = acct.ID
		}

		// Let's also resume tracking payments that have a last recorded
		// state of being in-flight.
		for hash, entry := range acct.Payments {
			entry := entry
			if !successState(entry.Status) {
				err := s.TrackPayment(
					ctx, acct.ID, hash, entry.FullAmount,
				)
				if err != nil {
					s.disable()
					return fmt.Errorf("error "+
						"tracking payment: %w", err)
				}
			}
		}
	}

	// First ask our DB about the highest indexes we know. If this is the
	// first startup then the ErrNoInvoiceIndexKnown error is returned, and
	// we know we need to do a lookup.
	s.currentAddIndex, s.currentSettleIndex, err = s.store.LastIndexes(ctx)
	switch err {
	case nil:
		// All good, we stored indexes in the DB, use those values.

	case ErrNoInvoiceIndexKnown:
		// We don't have any invoice indexes stored yet, so this must be
		// our first startup. We only care about new invoices being
		// settled as those could potentially be payments to accounts.
		// We don't care about existing invoices since we only get here
		// if we start up the account system for the first time and
		// there are no accounts yet. We don't really care about new
		// invoices being added either since we'll inspect the RPC call
		// in the interceptor if a new invoice is created by an account.
		// Therefore, we only really care about future, settled
		// invoices, which the subscription will deliver to us.
		s.currentAddIndex = 0
		s.currentSettleIndex = 0

	default:
		s.disable()
		return fmt.Errorf("error determining last invoice indexes: %w",
			err)
	}

	invoiceChan, invoiceErrChan, err := lightningClient.SubscribeInvoices(
		ctx, lndclient.InvoiceSubscriptionRequest{
			AddIndex:    s.currentAddIndex,
			SettleIndex: s.currentSettleIndex,
		},
	)
	if err != nil {
		s.disable()
		return fmt.Errorf("error subscribing invoices: %w", err)
	}

	s.wg.Add(1)
	go s.handleForever(ctx, invoiceChan, invoiceErrChan)

	return nil
}

// Stop shuts down the account service.
func (s *InterceptorService) Stop() error {
	// We need to lock the request mutex to ensure that we don't stop the
	// service while we're processing a request.
	// This is especially important to ensure that we don't stop the service
	// exactly after a user has made an rpc call to send a payment we can't
	// know the payment hash for prior to the actual payment being sent
	// (i.e. Keysend or SendToRoute). This is because if we stop the service
	// after the send request has been sent to lnd, but before TrackPayment
	// has been called, we won't be able to track the payment and debit the
	// account.
	s.requestMtx.Lock()
	defer s.requestMtx.Unlock()

	s.cancel.WhenSome(func(cancel context.CancelFunc) { cancel() })
	close(s.quit)

	s.wg.Wait()

	return s.store.Close()
}

func (s *InterceptorService) trackPayment(ctx context.Context,
	req *trackedPayment) error {

	hash := req.hash

	// And start the long-running TrackPayment RPC.
	ctxc, cancel := context.WithCancel(ctx)
	req.cancel = fn.Some(cancel)
	statusChan, errChan, err := s.routerClient.TrackPayment(ctxc, hash)
	if err != nil {
		cancel()
		return err
	}

	// We're now tracking the call, store everything we need to be able to
	// cancel the streaming RPC.
	s.pendingPayments[hash] = req

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()

		for {
			select {
			case paymentUpdate := <-statusChan:
				terminalState, err := s.paymentUpdate(
					ctxc, hash, paymentUpdate,
				)
				if err != nil {
					s.mainErrCallback(err)
					return
				}

				if terminalState {
					return
				}

			case err := <-errChan:
				// If the payment wasn't initiated, we can't
				// track it really. We'll try again on next
				// startup, to make sure we don't miss any
				// payments.
				if errors.Is(
					err, channeldb.ErrPaymentNotInitiated,
				) {

					log.Debugf("Payment %v not initiated, "+
						"stopping tracking", hash)

					// We also remove the payment from the
					// account, so that the payment won't be
					// seen as in-flight balance when
					// calculating the account's available
					// balance.
					err := s.RemovePayment(ctx, hash)
					if err != nil {
						// We don't disable the service
						// here, as the worst that can
						// happen is that the payment is
						// seen as still in-flight.
						s.mainErrCallback(err)
					}

					return
				}

				if err != nil {
					// If we error when tracking the
					// payment, we stop the service.
					s.disable()
					err = fmt.Errorf("received "+
						"error from TrackPayment RPC "+
						"for payment %v: %w", hash, err)

					s.mainErrCallback(err)
				}
				return

			case <-ctxc.Done():
				return

			case <-s.quit:
				return
			}
		}
	}()

	return nil
}

func (s *InterceptorService) trackForever(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case req := <-s.trackPaymentReqs:
			req.errChan <- s.trackPayment(ctx, req)

		case <-ctx.Done():
			return

		case <-s.quit:
			return
		}
	}
}

func (s *InterceptorService) handleForever(ctx context.Context,
	invoiceChan <-chan *lndclient.Invoice,
	invoiceErrChan <-chan error) {

	defer s.wg.Done()

	for {
		select {
		case invoice := <-invoiceChan:
			// Don't panic if the invoice channel is closed.
			if invoice == nil {
				log.Infof("Invoice subscription closed")
				return
			}

			if err := s.invoiceUpdate(ctx, invoice); err != nil {
				log.Errorf("Error processing invoice "+
					"update: %v", err)

				s.mainErrCallback(err)
				return
			}

		case err := <-invoiceErrChan:
			// If the invoice subscription errors out, we
			// stop the service as we won't be able to
			// process invoices.
			s.disable()

			s.mainErrCallback(fmt.Errorf("error in invoice "+
				"subscription: %w", err))
			return

		case <-ctx.Done():
			return

		case <-s.quit:
			return
		}
	}
}

// IsRunning checks if the account service is running, and returns a boolean
// indicating whether it is running or not.
func (s *InterceptorService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.isEnabled
}

// isRunningUnsafe checks if the account service is running, and returns a
// boolean indicating whether it is running or not
//
// NOTE: The store lock MUST be held as either a read or write lock when calling
// this method.
func (s *InterceptorService) isRunningUnsafe() bool {
	return s.isEnabled
}

// disable disables the account service, and marks the service as not running.
// The function acquires the store write lock before disabling the service.
func (s *InterceptorService) disable() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.disableUnsafe()
}

// disableAndErrorfUnsafe disables the account service, and marks the service as
// not running.
//
// NOTE: The store lock MUST be held when calling this method.
func (s *InterceptorService) disableUnsafe() {
	s.isEnabled = false
}

// NewAccount creates a new OffChainBalanceAccount with the given balance and a
// randomly chosen ID.
func (s *InterceptorService) NewAccount(ctx context.Context,
	balance lnwire.MilliSatoshi,
	expirationDate time.Time, label string) (*OffChainBalanceAccount,
	error) {

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.store.NewAccount(ctx, balance, expirationDate, label)
}

// UpdateAccount writes an account to the database, overwriting the existing one
// if it exists.
func (s *InterceptorService) UpdateAccount(ctx context.Context,
	accountID AccountID, accountBalance,
	expirationDate int64) (*OffChainBalanceAccount, error) {

	s.mu.Lock()
	defer s.mu.Unlock()

	// As this function updates account balances, we require that the
	// service is running before we execute it.
	if !s.isRunningUnsafe() {
		// This case can only happen if the service is disabled while
		// we we're processing a request.
		return nil, ErrAccountServiceDisabled
	}

	// If the expiration date was set, parse it as a unix time stamp. A
	// value of -1 signals "don't update the expiration date".
	var expiry fn.Option[time.Time]
	if expirationDate > 0 {
		expiry = fn.Some(time.Unix(expirationDate, 0))
	} else if expirationDate == 0 {
		// Setting the expiration to 0 means don't expire in which case
		// we use a zero time (zero unix time would still be 1970, so
		// that doesn't work for us).
		expiry = fn.Some(time.Time{})
	}

	// If the new account balance was set, parse it as millisatoshis. A
	// value of -1 signals "don't update the balance".
	var balance fn.Option[int64]
	if accountBalance >= 0 {
		// Convert from satoshis to millisatoshis for storage.
		balance = fn.Some(int64(accountBalance) * 1000)
	}

	// Create the actual account in the macaroon account store.
	err := s.store.UpdateAccountBalanceAndExpiry(
		ctx, accountID, balance, expiry,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to update account: %w", err)
	}

	return s.store.Account(ctx, accountID)
}

// Account retrieves an account from the bolt DB and un-marshals it. If the
// account cannot be found, then ErrAccNotFound is returned.
func (s *InterceptorService) Account(ctx context.Context,
	id AccountID) (*OffChainBalanceAccount, error) {

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.Account(ctx, id)
}

// Accounts retrieves all accounts from the bolt DB and un-marshals them.
func (s *InterceptorService) Accounts(ctx context.Context) (
	[]*OffChainBalanceAccount, error) {

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.Accounts(ctx)
}

// RemoveAccount finds an account by its ID and removes it from the DB.
func (s *InterceptorService) RemoveAccount(ctx context.Context,
	id AccountID) error {

	s.mu.Lock()
	defer s.mu.Unlock()

	// Are we currently tracking any payments?
	for hash, payment := range s.pendingPayments {
		if payment.accountID != id {
			continue
		}

		// Let's remove the payment (which also cancels the tracking).
		err := s.removePayment(ctx, hash, lnrpc.Payment_FAILED)
		if err != nil {
			return err
		}
	}

	return s.store.RemoveAccount(ctx, id)
}

// CheckBalance ensures an account is valid and has a balance equal to or larger
// than the amount that is required.
func (s *InterceptorService) CheckBalance(ctx context.Context, id AccountID,
	requiredBalance lnwire.MilliSatoshi) error {

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check that the account exists, it hasn't expired and has sufficient
	// balance.
	account, err := s.store.Account(ctx, id)
	if err != nil {
		return err
	}

	if account.HasExpired() {
		return ErrAccExpired
	}

	availableAmount := calcAvailableAccountBalance(account)
	if availableAmount < int64(requiredBalance) {
		return ErrAccBalanceInsufficient
	}

	return nil
}

func calcAvailableAccountBalance(account *OffChainBalanceAccount) int64 {
	var inFlightAmt int64
	for _, payment := range account.Payments {
		if inflightState(payment.Status) {
			// If a payment is in-flight and associated with the
			// account, the user should not be able to spend that
			// amount while it's in-flight.
			inFlightAmt += int64(payment.FullAmount)
		}
	}

	return account.CurrentBalance - inFlightAmt
}

// AssociateInvoice associates a generated invoice with the given account,
// making it possible for the account to be credited in case the invoice is
// paid.
func (s *InterceptorService) AssociateInvoice(ctx context.Context, id AccountID,
	hash lntypes.Hash) error {

	s.mu.Lock()
	defer s.mu.Unlock()

	s.invoiceToAccount[hash] = id

	return s.store.AddAccountInvoice(ctx, id, hash)
}

// PaymentErrored removes a pending payment from the account's registered
// payment list. This should only ever be called if we are sure that the payment
// request errored out.
func (s *InterceptorService) PaymentErrored(ctx context.Context, id AccountID,
	hash lntypes.Hash) error {

	s.mu.Lock()
	defer s.mu.Unlock()

	// If we have already started tracking this payment, then RemovePayment
	// should have been called instead.
	_, ok := s.pendingPayments[hash]
	if ok {
		return fmt.Errorf("cannot disassociate payment if tracking " +
			"has already started")
	}

	return s.store.SetAccountPaymentErrored(ctx, id, hash)
}

// AssociatePayment associates a payment (hash) with the given account,
// ensuring that the payment will be tracked for a user when LiT is
// restarted.
func (s *InterceptorService) AssociatePayment(ctx context.Context, id AccountID,
	paymentHash lntypes.Hash, fullAmt lnwire.MilliSatoshi) error {

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.store.AddAccountPayment(ctx, id, paymentHash, fullAmt)
}

// invoiceUpdate credits the account an invoice was registered with, in case the
// invoice was settled.
//
// NOTE: Any code that errors in this function MUST call disableAndErrorfUnsafe
// while the store lock is held to ensure that the service is disabled under
// the same lock. Else we risk that other threads will try to update invoices
// while the service should be disabled, which could lead to us missing invoice
// updates on next startup.
func (s *InterceptorService) invoiceUpdate(ctx context.Context,
	invoice *lndclient.Invoice) error {

	s.mu.Lock()
	defer s.mu.Unlock()

	// As this function updates account balances, and is called from the
	// invoice subscription, we ensure that the service is running before we
	// execute it.
	if !s.isRunningUnsafe() {
		// We will process the invoice update on next startup instead,
		// once the error that caused the service to stop has been
		// resolved.
		return ErrAccountServiceDisabled
	}

	// We update our indexes each time we get a new invoice from our
	// subscription. This might be a bit inefficient but makes sure we don't
	// miss an update.
	needUpdate := false
	if invoice.AddIndex > s.currentAddIndex {
		s.currentAddIndex = invoice.AddIndex
		needUpdate = true
	}
	if invoice.SettleIndex > s.currentSettleIndex {
		s.currentSettleIndex = invoice.SettleIndex
		needUpdate = true
	}

	if needUpdate {
		err := s.store.StoreLastIndexes(
			ctx, s.currentAddIndex, s.currentSettleIndex,
		)
		if err != nil {
			s.disableUnsafe()

			return fmt.Errorf("error storing last indexes: %w", err)
		}
	}

	// The invoice hasn't been settled yet, there is nothing for us to do.
	// If it eventually settles, we'll be called again.
	if invoice.State != invpkg.ContractSettled {
		return nil
	}

	// The invoice was settled, let's now credit the account. But only if
	// the invoice actually belongs to an account that we track.
	acctID, ok := s.invoiceToAccount[invoice.Hash]
	if !ok {
		return nil
	}

	// If we get here, the current account has the invoice associated with
	// it that was just paid. Credit the amount to the account and update it
	// in the DB.
	err := s.store.IncreaseAccountBalance(ctx, acctID, invoice.AmountPaid)
	if err != nil {
		s.disableUnsafe()
		return fmt.Errorf("error updating account: %w", err)
	}

	// We've now fully processed the invoice and don't need to keep it
	// mapped in memory anymore.
	delete(s.invoiceToAccount, invoice.Hash)

	return nil
}

// TrackPayment adds a new payment to be tracked to the service. If the payment
// is eventually settled, its amount needs to be debited from the given account.
func (s *InterceptorService) TrackPayment(ctx context.Context, id AccountID,
	hash lntypes.Hash, fullAmt lnwire.MilliSatoshi) error {

	s.mu.Lock()
	defer s.mu.Unlock()

	// Are we already tracking the payment? Then ignore the call. This might
	// happen because of the way we receive RPC updates.
	if _, ok := s.pendingPayments[hash]; ok {
		return nil
	}

	known, err := s.store.UpsertAccountPayment(ctx, id, hash, fullAmt)
	if err != nil {
		if !known {
			// In the rare case that the payment isn't associated
			// with an account yet, and we fail to update the
			// account we will not be tracking the payment, even if
			// track the service is restarted. Therefore the node
			// runner needs to manually check if the payment was
			// made and debit the account if that's the case.
			errStr := "critical error: failed to store the " +
				"payment with hash %v for user with account " +
				"id %v. Manual intervention required! " +
				"Verify if the payment was executed, and " +
				"manually update the user account balance by " +
				"subtracting the payment amount if it was"

			s.disableUnsafe()

			s.mainErrCallback(fmt.Errorf(errStr, hash, id))
		}

		return fmt.Errorf("error updating account: %w", err)
	}

	// As this function updates account balances, we ensure that the service
	// is running before we execute it.
	if !s.isRunningUnsafe() {
		// We will track the payment on next on next startup instead,
		// once the error that caused the service to stop has been
		// resolved.
		return ErrAccountServiceDisabled
	}

	errChan := make(chan error, 1)
	s.trackPaymentReqs <- &trackedPayment{
		accountID:  id,
		hash:       hash,
		fullAmount: fullAmt,
		errChan:    errChan,
	}

	select {
	case err := <-errChan:
		return err
	case <-s.quit:
		return fmt.Errorf("service stopped while initiating " +
			"payment tracking")
	case <-ctx.Done():
		return fmt.Errorf("caller context cancelled: %w", ctx.Err())
	}
}

// paymentUpdate debits the full amount of a payment from the account it was
// associated with, in case it is settled. The boolean value returned indicates
// whether the status was terminal or not. If it's not terminal then further
// updates are expected.
//
// NOTE: Any code that errors in this function MUST call disableAndErrorfUnsafe
// while the store lock is held to ensure that the service is disabled under
// the same lock.
func (s *InterceptorService) paymentUpdate(ctx context.Context,
	hash lntypes.Hash, status lndclient.PaymentStatus) (bool, error) {

	// Are we still in-flight? Then we don't have to do anything just yet.
	// The unknown state should never happen in practice but if it ever did
	// we couldn't handle it anyway, so let's also ignore it.
	if inflightState(status.State) {
		return false, nil
	}

	// Any other state is terminal, so whatever happens, we don't need to
	// keep waiting for more updates.
	const terminalState = true

	s.mu.Lock()
	defer s.mu.Unlock()

	// As this function updates account balances, we ensure that the service
	// is running before we execute it.
	if !s.isRunningUnsafe() {
		// We will update the payment on next startup instead, once the
		// error that caused the service to stop has been resolved.
		return false, ErrAccountServiceDisabled
	}

	pendingPayment, ok := s.pendingPayments[hash]
	if !ok {
		s.disableUnsafe()

		return terminalState, fmt.Errorf("payment %x not mapped to "+
			"any account", hash[:])
	}

	// A failed payment can just be removed, no further action needed.
	if status.State == lnrpc.Payment_FAILED {
		err := s.removePayment(ctx, hash, status.State)
		if err != nil {
			s.disableUnsafe()
			err = fmt.Errorf("error removing payment: %w", err)
		}

		return terminalState, err
	}

	// The payment went through! We now need to debit the full amount from
	// the account.
	fullAmount := status.Value + status.Fee

	err := s.store.UpdateAccountPaymentSuccess(
		ctx, pendingPayment.accountID, hash, fullAmount,
	)
	if err != nil {
		s.disableUnsafe()
		return terminalState, fmt.Errorf("error updating account: %w",
			err)
	}

	// We've now fully processed the payment and don't need to keep it
	// mapped or tracked anymore.
	err = s.removePayment(ctx, hash, lnrpc.Payment_SUCCEEDED)
	if err != nil {
		s.disableUnsafe()

		err = fmt.Errorf("error removing payment: %w", err)
	}

	return terminalState, err
}

// RemovePayment removes a failed payment from the service because it no longer
// needs to be tracked. The payment is certain to never succeed, so we never
// need to debit the amount from the account.
func (s *InterceptorService) RemovePayment(ctx context.Context,
	hash lntypes.Hash) error {

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.removePayment(ctx, hash, lnrpc.Payment_FAILED)
}

// removePayment stops tracking a payment and updates the status in the account
// to the given status.
//
// NOTE: The store lock MUST be held when calling this method.
func (s *InterceptorService) removePayment(ctx context.Context,
	hash lntypes.Hash, status lnrpc.Payment_PaymentStatus) error {

	// It could be that we haven't actually started tracking the payment
	// yet, so if we can't find it, we just do nothing.
	pendingPayment, ok := s.pendingPayments[hash]
	if !ok {
		return nil
	}

	pendingPayment.cancel.WhenSome(func(fn context.CancelFunc) { fn() })
	delete(s.pendingPayments, hash)

	return s.store.UpdateAccountPaymentStatus(
		ctx, pendingPayment.accountID, hash, status,
	)
}

// successState returns true if a payment was completed successfully.
func successState(status lnrpc.Payment_PaymentStatus) bool {
	return status == lnrpc.Payment_SUCCEEDED
}

// inflightState returns true if a payment should be seen as in-flight by the
// accounts system.
func inflightState(status lnrpc.Payment_PaymentStatus) bool {
	return status != lnrpc.Payment_SUCCEEDED &&
		status != lnrpc.Payment_FAILED
}

// requestValuesStore is an in-memory implementation of the
// RequestValuesStore interface.
type requestValuesStore struct {
	m map[uint64]*RequestValues

	mu sync.Mutex
}

// A compile-time check to ensure that requestValuesStore implements the
// RequestValuesStore interface.
var _ RequestValuesStore = (*requestValuesStore)(nil)

// newRequestValuesStore constructs a new requestValuesStore which is an
// implementation of the RequestValuesStore interface.
func newRequestValuesStore() *requestValuesStore {
	return &requestValuesStore{
		m: make(map[uint64]*RequestValues),
	}
}

// RegisterValues stores values for the given request ID.
func (s *requestValuesStore) RegisterValues(reqID uint64,
	values *RequestValues) error {

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.m[reqID]; ok {
		return fmt.Errorf("values for request ID %d have already "+
			"been registered", reqID)
	}

	s.m[reqID] = values

	return nil
}

// GetValues returns the corresponding request values for the given request ID
// if they exist.
func (s *requestValuesStore) GetValues(reqID uint64) (*RequestValues, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	values, ok := s.m[reqID]

	return values, ok
}

// DeleteValues deletes any values stored for the given request ID.
func (s *requestValuesStore) DeleteValues(reqID uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.m, reqID)
}

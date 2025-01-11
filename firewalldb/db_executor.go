package firewalldb

import (
	"context"

	"github.com/lightninglabs/lightning-terminal/db"
)

// DBExecutor provides an Update and View method that will allow the caller
// to perform atomic read and write transactions defined by PrivacyMapTx on the
// underlying DB.
type DBExecutor[T any] interface {
	// Update opens a database read/write transaction and executes the
	// function f with the transaction passed as a parameter. After f exits,
	// if f did not error, the transaction is committed. Otherwise, if f did
	// error, the transaction is rolled back. If the rollback fails, the
	// original error returned by f is still returned. If the commit fails,
	// the commit error is returned.
	Update(ctx context.Context, f func(tx T) error) error

	// View opens a database read transaction and executes the function f
	// with the transaction passed as a parameter. After f exits, the
	// transaction is rolled back. If f errors, its error is returned, not a
	// rollback error (if any occur).
	View(ctx context.Context, f func(tx T) error) error
}

type txCreator[T db.Tx] interface {
	beginTx(ctx context.Context, writable bool) (T, error)
}

type NullableTx interface {
	db.Tx
	IsNil() bool
}

// dbExecutor is an implementation of DBExecutor.
type dbExecutor[T NullableTx] struct {
	db txCreator[T]
}

// Update opens a database read/write transaction and executes the function f
// with the transaction passed as a parameter. After f exits, if f did not
// error, the transaction is committed. Otherwise, if f did error, the
// transaction is rolled back. If the rollback fails, the original error
// returned by f is still returned. If the commit fails, the commit error is
// returned.
//
// NOTE: this is part of the DBExecutor interface.
func (p *dbExecutor[T]) Update(ctx context.Context,
	f func(tx T) error) error {

	tx, err := p.db.beginTx(ctx, true)
	if err != nil {
		return err
	}

	// Make sure the transaction rolls back in the event of a panic.
	defer func() {
		if !tx.IsNil() {
			_ = tx.Rollback()
		}
	}()

	err = f(tx)
	if err != nil {
		// Want to return the original error, not a rollback error if
		// any occur.
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// View opens a database read transaction and executes the function f with the
// transaction passed as a parameter. After f exits, the transaction is rolled
// back. If f errors, its error is returned, not a rollback error (if any
// occur).
//
// NOTE: this is part of the DBExecutor interface.
func (p *dbExecutor[T]) View(ctx context.Context,
	f func(tx T) error) error {

	tx, err := p.db.beginTx(ctx, false)
	if err != nil {
		return err
	}

	// Make sure the transaction rolls back in the event of a panic.
	defer func() {
		if !tx.IsNil() {
			_ = tx.Rollback()
		}
	}()

	err = f(tx)
	rollbackErr := tx.Rollback()
	if err != nil {
		return err
	}

	if rollbackErr != nil {
		return rollbackErr
	}
	return nil
}

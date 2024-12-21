// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.25.0
// source: accounts.sql

package sqlc

import (
	"context"
	"database/sql"
	"time"
)

const addAccountInvoice = `-- name: AddAccountInvoice :exec
INSERT INTO account_invoices (account_id, hash)
SELECT id, $1
FROM accounts
WHERE alias = $2
`

type AddAccountInvoiceParams struct {
	Hash  []byte
	Alias []byte
}

func (q *Queries) AddAccountInvoice(ctx context.Context, arg AddAccountInvoiceParams) error {
	_, err := q.db.ExecContext(ctx, addAccountInvoice, arg.Hash, arg.Alias)
	return err
}

const deleteAccount = `-- name: DeleteAccount :exec
DELETE FROM accounts
WHERE alias = $1
`

func (q *Queries) DeleteAccount(ctx context.Context, alias []byte) error {
	_, err := q.db.ExecContext(ctx, deleteAccount, alias)
	return err
}

const deleteAccountPayment = `-- name: DeleteAccountPayment :exec
DELETE FROM account_payments
WHERE hash = $1
  AND account_id = (SELECT id FROM accounts WHERE alias = $2)
`

type DeleteAccountPaymentParams struct {
	Hash  []byte
	Alias []byte
}

func (q *Queries) DeleteAccountPayment(ctx context.Context, arg DeleteAccountPaymentParams) error {
	_, err := q.db.ExecContext(ctx, deleteAccountPayment, arg.Hash, arg.Alias)
	return err
}

const getAccount = `-- name: GetAccount :one
SELECT id, alias, label, type, intial_balance_msat, current_balance_msat, last_updated, expiration
FROM accounts
WHERE alias = $1
`

func (q *Queries) GetAccount(ctx context.Context, alias []byte) (Account, error) {
	row := q.db.QueryRowContext(ctx, getAccount, alias)
	var i Account
	err := row.Scan(
		&i.ID,
		&i.Alias,
		&i.Label,
		&i.Type,
		&i.IntialBalanceMsat,
		&i.CurrentBalanceMsat,
		&i.LastUpdated,
		&i.Expiration,
	)
	return i, err
}

const getAccountByAliasPrefix = `-- name: GetAccountByAliasPrefix :one
SELECT id, alias, label, type, intial_balance_msat, current_balance_msat, last_updated, expiration
FROM accounts
WHERE SUBSTR(alias, 1, 4) = $1
`

func (q *Queries) GetAccountByAliasPrefix(ctx context.Context, alias []byte) (Account, error) {
	row := q.db.QueryRowContext(ctx, getAccountByAliasPrefix, alias)
	var i Account
	err := row.Scan(
		&i.ID,
		&i.Alias,
		&i.Label,
		&i.Type,
		&i.IntialBalanceMsat,
		&i.CurrentBalanceMsat,
		&i.LastUpdated,
		&i.Expiration,
	)
	return i, err
}

const getAccountByID = `-- name: GetAccountByID :one
SELECT id, alias, label, type, intial_balance_msat, current_balance_msat, last_updated, expiration
FROM accounts
WHERE id = $1
`

func (q *Queries) GetAccountByID(ctx context.Context, id int64) (Account, error) {
	row := q.db.QueryRowContext(ctx, getAccountByID, id)
	var i Account
	err := row.Scan(
		&i.ID,
		&i.Alias,
		&i.Label,
		&i.Type,
		&i.IntialBalanceMsat,
		&i.CurrentBalanceMsat,
		&i.LastUpdated,
		&i.Expiration,
	)
	return i, err
}

const getAccountByLabel = `-- name: GetAccountByLabel :one
SELECT id, alias, label, type, intial_balance_msat, current_balance_msat, last_updated, expiration
FROM accounts
WHERE label = $1
`

func (q *Queries) GetAccountByLabel(ctx context.Context, label sql.NullString) (Account, error) {
	row := q.db.QueryRowContext(ctx, getAccountByLabel, label)
	var i Account
	err := row.Scan(
		&i.ID,
		&i.Alias,
		&i.Label,
		&i.Type,
		&i.IntialBalanceMsat,
		&i.CurrentBalanceMsat,
		&i.LastUpdated,
		&i.Expiration,
	)
	return i, err
}

const getAccountIndex = `-- name: GetAccountIndex :one
SELECT value
FROM account_indicies
WHERE name = $1
`

func (q *Queries) GetAccountIndex(ctx context.Context, name string) (int64, error) {
	row := q.db.QueryRowContext(ctx, getAccountIndex, name)
	var value int64
	err := row.Scan(&value)
	return value, err
}

const getAccountPayment = `-- name: GetAccountPayment :one
SELECT ap.account_id, ap.hash, ap.status, ap.full_amount_msat
FROM account_payments ap
         JOIN accounts a ON ap.account_id = a.id
WHERE ap.hash = $1
  AND a.alias = $2
`

type GetAccountPaymentParams struct {
	Hash  []byte
	Alias []byte
}

func (q *Queries) GetAccountPayment(ctx context.Context, arg GetAccountPaymentParams) (AccountPayment, error) {
	row := q.db.QueryRowContext(ctx, getAccountPayment, arg.Hash, arg.Alias)
	var i AccountPayment
	err := row.Scan(
		&i.AccountID,
		&i.Hash,
		&i.Status,
		&i.FullAmountMsat,
	)
	return i, err
}

const insertAccount = `-- name: InsertAccount :one
INSERT INTO accounts (type, intial_balance_msat, current_balance_msat, last_updated, label, alias, expiration)
VALUES ($1, $2, $3, $4, $5, $6, $7)
    RETURNING id
`

type InsertAccountParams struct {
	Type               int16
	IntialBalanceMsat  int64
	CurrentBalanceMsat int64
	LastUpdated        time.Time
	Label              sql.NullString
	Alias              []byte
	Expiration         time.Time
}

func (q *Queries) InsertAccount(ctx context.Context, arg InsertAccountParams) (int64, error) {
	row := q.db.QueryRowContext(ctx, insertAccount,
		arg.Type,
		arg.IntialBalanceMsat,
		arg.CurrentBalanceMsat,
		arg.LastUpdated,
		arg.Label,
		arg.Alias,
		arg.Expiration,
	)
	var id int64
	err := row.Scan(&id)
	return id, err
}

const listAccountInvoices = `-- name: ListAccountInvoices :many
SELECT ai.account_id, ai.hash
FROM account_invoices ai
         JOIN accounts a ON ai.account_id = a.id
WHERE a.alias = $1
`

func (q *Queries) ListAccountInvoices(ctx context.Context, alias []byte) ([]AccountInvoice, error) {
	rows, err := q.db.QueryContext(ctx, listAccountInvoices, alias)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []AccountInvoice
	for rows.Next() {
		var i AccountInvoice
		if err := rows.Scan(&i.AccountID, &i.Hash); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listAccountPayments = `-- name: ListAccountPayments :many
SELECT ap.account_id, ap.hash, ap.status, ap.full_amount_msat
FROM account_payments ap
         JOIN accounts a ON ap.account_id = a.id
WHERE a.alias = $1
`

func (q *Queries) ListAccountPayments(ctx context.Context, alias []byte) ([]AccountPayment, error) {
	rows, err := q.db.QueryContext(ctx, listAccountPayments, alias)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []AccountPayment
	for rows.Next() {
		var i AccountPayment
		if err := rows.Scan(
			&i.AccountID,
			&i.Hash,
			&i.Status,
			&i.FullAmountMsat,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listAllAccountIDs = `-- name: ListAllAccountIDs :many
SELECT alias
FROM accounts
`

func (q *Queries) ListAllAccountIDs(ctx context.Context) ([][]byte, error) {
	rows, err := q.db.QueryContext(ctx, listAllAccountIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items [][]byte
	for rows.Next() {
		var alias []byte
		if err := rows.Scan(&alias); err != nil {
			return nil, err
		}
		items = append(items, alias)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const setAccountIndex = `-- name: SetAccountIndex :exec
INSERT INTO account_indicies (name, value)
VALUES ($1, $2)
    ON CONFLICT (name)
DO UPDATE SET value = $2
`

type SetAccountIndexParams struct {
	Name  string
	Value int64
}

func (q *Queries) SetAccountIndex(ctx context.Context, arg SetAccountIndexParams) error {
	_, err := q.db.ExecContext(ctx, setAccountIndex, arg.Name, arg.Value)
	return err
}

const updateAccountBalance = `-- name: UpdateAccountBalance :exec
UPDATE accounts
SET current_balance_msat = $1
WHERE alias = $2
`

type UpdateAccountBalanceParams struct {
	CurrentBalanceMsat int64
	Alias              []byte
}

func (q *Queries) UpdateAccountBalance(ctx context.Context, arg UpdateAccountBalanceParams) error {
	_, err := q.db.ExecContext(ctx, updateAccountBalance, arg.CurrentBalanceMsat, arg.Alias)
	return err
}

const updateAccountExpiry = `-- name: UpdateAccountExpiry :exec
UPDATE accounts
SET expiration = $1
WHERE alias = $2
`

type UpdateAccountExpiryParams struct {
	Expiration time.Time
	Alias      []byte
}

func (q *Queries) UpdateAccountExpiry(ctx context.Context, arg UpdateAccountExpiryParams) error {
	_, err := q.db.ExecContext(ctx, updateAccountExpiry, arg.Expiration, arg.Alias)
	return err
}

const updateAccountLastUpdate = `-- name: UpdateAccountLastUpdate :exec
UPDATE accounts
SET last_updated = $1
WHERE alias = $2
`

type UpdateAccountLastUpdateParams struct {
	LastUpdated time.Time
	Alias       []byte
}

func (q *Queries) UpdateAccountLastUpdate(ctx context.Context, arg UpdateAccountLastUpdateParams) error {
	_, err := q.db.ExecContext(ctx, updateAccountLastUpdate, arg.LastUpdated, arg.Alias)
	return err
}

const upsertAccountPayment = `-- name: UpsertAccountPayment :exec
INSERT INTO account_payments (account_id, hash, status, full_amount_msat)
VALUES (
           (SELECT id FROM accounts WHERE alias = $1),
           $2, $3, $4
       )
ON CONFLICT (account_id, hash)
DO UPDATE SET status = $3, full_amount_msat = $4
`

type UpsertAccountPaymentParams struct {
	Alias          []byte
	Hash           []byte
	Status         int16
	FullAmountMsat int64
}

func (q *Queries) UpsertAccountPayment(ctx context.Context, arg UpsertAccountPaymentParams) error {
	_, err := q.db.ExecContext(ctx, upsertAccountPayment,
		arg.Alias,
		arg.Hash,
		arg.Status,
		arg.FullAmountMsat,
	)
	return err
}

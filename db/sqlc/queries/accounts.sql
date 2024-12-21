-- name: InsertAccount :one
INSERT INTO accounts (type, intial_balance_msat, current_balance_msat, last_updated, label, alias, expiration)
VALUES ($1, $2, $3, $4, $5, $6, $7)
    RETURNING id;

-- name: UpdateAccountBalance :one
UPDATE accounts
SET current_balance_msat = $1
WHERE id = $2
RETURNING id;

-- name: UpdateAccountExpiry :one
UPDATE accounts
SET expiration = $1
where id = $2
RETURNING id;

-- name: UpdateAccountLastUpdate :one
UPDATE accounts
SET last_updated = $1
WHERE id = $2
RETURNING id;

-- name: AddAccountInvoice :exec
INSERT INTO account_invoices (account_id, hash)
SELECT id, $1
FROM accounts
WHERE id = $2;

-- name: DeleteAccountPayment :exec
DELETE FROM account_payments
WHERE hash = $1
  AND account_id = (SELECT id FROM accounts WHERE id = $2);

-- name: UpsertAccountPayment :exec
INSERT INTO account_payments (account_id, hash, status, full_amount_msat)
VALUES (
           (SELECT id FROM accounts WHERE id = $1),
           $2, $3, $4
       )
ON CONFLICT (account_id, hash)
DO UPDATE SET status = $3, full_amount_msat = $4;

-- name: GetAccountPayment :one
SELECT ap.account_id, ap.hash, ap.status, ap.full_amount_msat
FROM account_payments ap
         JOIN accounts a ON ap.account_id = a.id
WHERE ap.hash = $1
  AND a.id = $2;

-- name: GetAccount :one
SELECT *
FROM accounts
WHERE id = $1;

-- name: GetAccountIDByAlias :one
SELECT id
FROM accounts
WHERE alias = $1;

-- name: GetAccountByLabel :one
SELECT *
FROM accounts
WHERE label = $1;

-- name: DeleteAccount :exec
DELETE FROM accounts
WHERE id = $1;

-- name: ListAllAccounts :many
SELECT *
FROM accounts;

-- name: ListAccountPayments :many
SELECT ap.*
FROM account_payments ap
         JOIN accounts a ON ap.account_id = a.id
WHERE a.id = $1;

-- name: ListAccountInvoices :many
SELECT *
FROM account_invoices
WHERE account_id = $1;

-- name: GetAccountInvoice :one
SELECT *
FROM  account_invoices
WHERE account_id = $1
  AND hash = $2;

-- name: SetAccountIndex :exec
INSERT INTO account_indicies (name, value)
VALUES ($1, $2)
    ON CONFLICT (name)
DO UPDATE SET value = $2;

-- name: GetAccountIndex :one
SELECT value
FROM account_indicies
WHERE name = $1;

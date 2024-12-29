-- name: InsertAccount :one
INSERT INTO accounts (type, intial_balance_msat, current_balance_msat, last_updated, label, legacy_id, expiration)
VALUES ($1, $2, $3, $4, $5, $6, $7)
    RETURNING id;

-- name: UpdateAccountBalance :exec
UPDATE accounts
SET current_balance_msat = $1
WHERE legacy_id = $2;

-- name: UpdateAccountExpiry :exec
UPDATE accounts
SET expiration = $1
WHERE legacy_id = $2;

-- name: UpdateAccountLastUpdate :exec
UPDATE accounts
SET last_updated = $1
WHERE legacy_id = $2;

-- name: AddAccountInvoice :exec
INSERT INTO account_invoices (account_id, hash)
SELECT id, $1
FROM accounts
WHERE legacy_id = $2;

-- name: DeleteAccountPayment :exec
DELETE FROM account_payments
WHERE hash = $1
  AND account_id = (SELECT id FROM accounts WHERE legacy_id = $2);

-- name: UpsertAccountPayment :exec
INSERT INTO account_payments (account_id, hash, status, full_amount_msat)
VALUES (
           (SELECT id FROM accounts WHERE legacy_id = $1),
           $2, $3, $4
       )
ON CONFLICT (account_id, hash)
DO UPDATE SET status = $3, full_amount_msat = $4;

-- name: GetAccountPayment :one
SELECT ap.account_id, ap.hash, ap.status, ap.full_amount_msat
FROM account_payments ap
         JOIN accounts a ON ap.account_id = a.id
WHERE ap.hash = $1
  AND a.legacy_id = $2;

-- name: GetAccount :one
SELECT *
FROM accounts
WHERE legacy_id = $1;

-- name: GetAccountByAliasPrefix :one
SELECT *
FROM accounts
WHERE SUBSTR(legacy_id, 1, 4) = $1;

-- name: GetAccountByID :one
SELECT *
FROM accounts
WHERE id = $1;

-- name: DeleteAccount :exec
DELETE FROM accounts
WHERE legacy_id = $1;

-- name: ListAllAccountIDs :many
SELECT legacy_id
FROM accounts;

-- name: ListAccountPayments :many
SELECT ap.*
FROM account_payments ap
         JOIN accounts a ON ap.account_id = a.id
WHERE a.legacy_id = $1;

-- name: ListAccountInvoices :many
SELECT ai.*
FROM account_invoices ai
         JOIN accounts a ON ai.account_id = a.id
WHERE a.legacy_id = $1;

-- name: SetAccountIndex :exec
INSERT INTO account_indicies (name, value)
VALUES ($1, $2)
    ON CONFLICT (name)
DO UPDATE SET value = $2;

-- name: GetAccountIndex :one
SELECT value
FROM account_indicies
WHERE name = $1;
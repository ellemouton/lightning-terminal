CREATE TABLE IF NOT EXISTS accounts(
    -- The auto incrementing primary key.
    id BIGINT PRIMARY KEY,

    -- The ID that was used to identify the account in the legacy KVDB store.
    -- In order to avoid breaking the API, we keep this field here so that
    -- we can still look up accounts by this ID for the time being.
    -- TODO: rename to alias.
    legacy_id BLOB NOT NULL UNIQUE,

    -- An optional label to use for the account. If it is set, it must be
    -- unique.
    label TEXT UNIQUE,

    -- The account type.
    type SMALLINT NOT NULL,

    -- The accounts initial balance. This is never updated.
    intial_balance_msat BIGINT NOT NULL,

    -- The accounts current balance. This is updated as the account is used.
    current_balance_msat BIGINT NOT NULL,

    -- The last time the account was updated.
    last_updated TIMESTAMP NOT NULL,

    -- The time that the account will expire.
    expiration TIMESTAMP NOT NULL
);

CREATE INDEX idx_accounts_alias_prefix ON accounts(SUBSTR(legacy_id, 1, 4));

CREATE TABLE IF NOT EXISTS account_payments (
    -- The account that this payment is linked to.
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    -- The payment hash of the payment.
    hash BLOB NOT NULL,

    -- The LND RPC status of the payment.
    status SMALLINT NOT NULL,

    -- The total amount of the payment in millisatoshis.
    -- This includes the payment amount and estimated routing fee.
    full_amount_msat BIGINT NOT NULL,

    UNIQUE(account_id, hash)
);

CREATE TABLE IF NOT EXISTS account_invoices (
    -- The account that this invoice is linked to.
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    -- The payment hash of the invoice.
    hash BLOB NOT NULL,

    UNIQUE(account_id, hash)
);

CREATE TABLE IF NOT EXISTS account_indicies (
    name TEXT NOT NULL UNIQUE,

    value BIGINT NOT NULL
);
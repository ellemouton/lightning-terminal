-- The sessions table contains LNC session related information.
CREATE TABLE IF NOT EXISTS sessions (
    -- The auto incrementing primary key.
    id BIGINT PRIMARY KEY,

    -- The ID that was used to identify the session in the legacy KVDB store.
    -- This is derived directly from the local_public_key. In order to avoid
    -- breaking the API, we keep this field here so that we can still look up
    -- sessions by this ID.
    legacy_id BLOB NOT NULL UNIQUE,

    -- The session's given label.
    label TEXT NOT NULL,

    -- The session's current state.
    state SMALLINT NOT NULL,

    -- The session type.
    type SMALLINT NOT NULL,

    -- expiry is the time that the session will expire.
    expiry TIMESTAMP NOT NULL,

    -- The session's creation time.
    created_at TIMESTAMP NOT NULL,

    -- The time at which the session was revoked.
    revoked_at TIMESTAMP,

    -- The mailbox server address.
    server_address TEXT NOT NULL,

    -- Whether the connection to the server should not use TLS.
    dev_server BOOLEAN NOT NULL,

    -- The root key ID to use when baking a macaroon for this session.
    macaroon_root_key BIGINT NOT NULL,

    -- The passphrase entropy to use when deriving the mnemonic for this LNC
    -- session.
    pairing_secret BLOB NOT NULL,

    -- The private key of the long term local static key for this LNC session.
    local_private_key BLOB NOT NULL,

    -- The public key of the long term local static key for this LNC session.
    -- This is derivable from the local_private_key but is stored here since
    -- the local public key was used to identify a session when the DB was KVDB
    -- based and so to keep the API consistent, we store it here so that we can
    -- still look up sessions by this public key.
    local_public_key BLOB NOT NULL UNIQUE,

    -- The public key of the long term remote static key for this LNC session.
    remote_public_key BLOB,

    -- Whether the privacy mapper should be used for this session.
    privacy BOOLEAN NOT NULL,

    account_id BIGINT REFERENCES accounts(id) ON DELETE CASCADE,

    -- The session ID of the first session in this linked session group. This
    -- is nullable for the case where the first session in the group is being
    -- inserted, and so we first need to insert the session before we know the
    -- ID to use for the group ID.
    group_id BIGINT REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS sessions_type_idx ON sessions(type);

-- The macaroon_permissions table contains the macaroon permissions that are
-- associated with a session.
CREATE TABLE IF NOT EXISTS macaroon_permissions (
    -- The ID of the session in the sessions table that this permission is
    -- associated with.
    session_id BIGINT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,

    -- The entity that this permission is for.
    entity TEXT NOT NULL,

    -- The action that this permission is for.
    action TEXT NOT NULL
);

-- The macaroon_caveats table contains the macaroon caveats that are associated
-- with a session.
CREATE TABLE IF NOT EXISTS macaroon_caveats (
    -- The ID of the session in the sessions table that this caveat is
    -- associated with.
    session_id BIGINT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,

    -- The caveat ID.
    id BLOB NOT NULL,

    -- The verification ID. If this is not-null, it's a third party caveat.
    verification_id BLOB,

    -- The location hint for third party caveats.
    location TEXT
);

-- The feature_configs table contains the feature configs that are associated
-- with a session.
CREATE TABLE IF NOT EXISTS feature_configs (
    -- The ID of the session in the sessions table that this feature config is
    -- associated with.
    session_id BIGINT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,

    -- The feature name.
    feature_name TEXT NOT NULL,

    -- The feature config blob.
    config BLOB
);

-- The privacy_flags table contains the privacy flags that are associated with
-- a session.
CREATE TABLE IF NOT EXISTS privacy_flags (
    -- The ID of the session in the sessions table that this privacy bit is
    -- associated with.
    session_id BIGINT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,

    -- The privacy flag bit.
    flag INTEGER NOT NULL,

    -- The flag bit is unique per session.
    UNIQUE (flag, session_id)
);
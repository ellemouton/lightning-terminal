CREATE TABLE IF NOT EXISTS kvstores (
    -- Whether this record is part of the permanent store.
    -- If false, it will be deleted on start-up.
    perm BOOLEAN NOT NULL,

    -- The rule that this kv_store belongs to.
    -- If only the rule name is set, then this kv_store is a global
    -- kv_store.
    -- TODO: insert these names in a table and make this a FK instead?
    --  or actually, just make it an enum... convert from/to string
    --  in CRUD code
    rule_name TEXT NOT NULL,

    -- The session ID that this kv_store belongs to.
    -- If this is set, then this kv_store is a session-specific
    -- kv_store for the given rule.
    session_id BIGINT REFERENCES sessions(id) ON DELETE CASCADE,

    -- The feature name that this kv_store belongs to.
    -- If this is set, then this kv_store is a feature-specific
    -- kvstore under the given session ID and rule name.
    -- If this is set, then session_id must also be set.
    feature_name TEXT,

    key TEXT NOT NULL,

    value BLOB NOT NULL
);

CREATE INDEX IF NOT EXISTS kvstores_rule_name_idx ON kvstores(rule_name);
CREATE INDEX IF NOT EXISTS kvstores_session_id_idx ON kvstores(session_id);
CREATE INDEX IF NOT EXISTS kvstores_feature_name_idx ON kvstores(feature_name);
CREATE INDEX IF NOT EXISTS kvstores_key_idx ON kvstores(key);
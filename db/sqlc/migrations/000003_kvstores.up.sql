-- rules holds the unique names of the various rules that are used
-- known to lit and used by the firewall db.
CREATE TABLE IF NOT EXISTS rules (
    -- The auto incrementing primary key.
    id INTEGER PRIMARY KEY,

    -- The unique name of the rule.
    name TEXT NOT NULL UNIQUE
);

-- features holds the unique names of the various features that
-- kvstores can be associated with.
CREATE TABLE IF NOT EXISTS features (
    -- The auto incrementing primary key.
    id INTEGER PRIMARY KEY,

    -- The unique name of the feature.
    name TEXT NOT NULL UNIQUE
);

-- kvstores houses key-value pairs under various namespaces determined
-- by the rule name, session ID, and feature name.
CREATE TABLE IF NOT EXISTS kvstores (
    -- The auto incrementing primary key.
    id INTEGER PRIMARY KEY,

    -- Whether this record is part of the permanent store.
    -- If false, it will be deleted on start-up.
    perm BOOLEAN NOT NULL,

    -- The rule that this kv_store belongs to.
    -- If only the rule name is set, then this kv_store is a global
    -- kv_store.
    rule_id BIGINT REFERENCES rules(id) NOT NULL,

    -- The session ID that this kv_store belongs to.
    -- If this is set, then this kv_store is a session-specific
    -- kv_store for the given rule.
    session_id BIGINT REFERENCES sessions(id) ON DELETE CASCADE,

    -- The feature name that this kv_store belongs to.
    -- If this is set, then this kv_store is a feature-specific
    -- kvstore under the given session ID and rule name.
    -- If this is set, then session_id must also be set.
    feature_id BIGINT REFERENCES features(id),

    -- The key of the entry.
    key TEXT NOT NULL,

    -- The value of the entry.
    value BLOB NOT NULL
);

CREATE INDEX IF NOT EXISTS kvstores_lookup_idx
    ON kvstores (key, rule_id, perm, session_id, feature_id);

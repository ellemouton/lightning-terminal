CREATE TABLE IF NOT EXISTS privacy_pairs (
    -- The group ID of the session that this action is associated with.
    group_id BIGINT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,

    -- The key of the privacy pair.
    real TEXT NOT NULL,

    -- The value of the privacy pair.
    pseudo TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS privacy_pairs_real_idx ON privacy_pairs(real);
CREATE INDEX IF NOT EXISTS privacy_pairs_pseudo_idx ON privacy_pairs(pseudo);

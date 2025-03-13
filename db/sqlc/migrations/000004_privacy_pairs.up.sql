-- privacy_pairs stores the privacy map pairs for a given session group.
CREATE TABLE IF NOT EXISTS privacy_pairs (
    -- The group ID of the session that this privacy pair is associated
    -- with.
    group_id BIGINT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,

    -- The key of the privacy pair.
    real TEXT NOT NULL,

    -- The value of the privacy pair.
    pseudo TEXT NOT NULL
);

-- We'll want easy access to the privacy pairs for a given group ID.
CREATE INDEX IF NOT EXISTS privacy_pairs_group_id_idx ON privacy_pairs(group_id);

-- There should be no duplicate real values for a given group ID.
CREATE UNIQUE INDEX privacy_pairs_unique_real ON privacy_pairs (
    group_id, real
);

-- There should be no duplicate pseudo values for a given group ID.
CREATE UNIQUE INDEX privacy_pairs_unique_pseudo ON privacy_pairs (
    group_id, pseudo
);


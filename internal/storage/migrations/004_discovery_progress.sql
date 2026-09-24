CREATE TABLE discovery_progress (
    session_id TEXT PRIMARY KEY REFERENCES sessions(id),
    last_sequence INTEGER NOT NULL CHECK (last_sequence >= 0),
    accepted_count INTEGER NOT NULL CHECK (accepted_count >= 0)
);

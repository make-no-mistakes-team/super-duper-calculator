CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    expires_at INTEGER NOT NULL
);

CREATE TABLE calculations (
    seq INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    request_id TEXT NOT NULL,
    expression TEXT NOT NULL,
    angle_unit TEXT NOT NULL,
    semantics_version TEXT NOT NULL,
    outcome_json TEXT NOT NULL,
    facts_json TEXT,
    created_at TEXT NOT NULL,
    UNIQUE (session_id, request_id)
);

CREATE INDEX calculations_by_session ON calculations (session_id, seq DESC);

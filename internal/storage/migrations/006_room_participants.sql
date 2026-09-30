CREATE TABLE room_participants (
    room_code TEXT NOT NULL,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE RESTRICT,
    public_id TEXT NOT NULL UNIQUE,
    alias TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (room_code, session_id),
    UNIQUE (room_code, alias)
);

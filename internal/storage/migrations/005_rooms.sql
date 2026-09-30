ALTER TABLE calculations ADD COLUMN room_code TEXT;
ALTER TABLE calculations ADD COLUMN room_publish INTEGER CHECK (room_publish IN (0, 1));
ALTER TABLE calculations ADD COLUMN publication_status TEXT NOT NULL DEFAULT 'private'
    CHECK (publication_status IN ('private', 'pending', 'published', 'unavailable'));
ALTER TABLE calculations ADD COLUMN public_event_id TEXT;

CREATE TABLE room_events (
    seq INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    calculation_id TEXT NOT NULL UNIQUE REFERENCES calculations(id) ON DELETE RESTRICT,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE RESTRICT,
    room_code TEXT NOT NULL,
    alias TEXT NOT NULL,
    expression TEXT NOT NULL,
    value TEXT NOT NULL,
    angle_unit TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX room_events_recent ON room_events (room_code, seq DESC);

CREATE TABLE room_counters (
    room_code TEXT PRIMARY KEY,
    published_calculations INTEGER NOT NULL DEFAULT 0 CHECK (published_calculations >= 0)
);

CREATE TABLE room_reactions (
    event_id TEXT NOT NULL REFERENCES room_events(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE RESTRICT,
    reaction_id TEXT NOT NULL,
    PRIMARY KEY (event_id, session_id)
);

CREATE TABLE room_badges (
    event_id TEXT NOT NULL REFERENCES room_events(id) ON DELETE CASCADE,
    rule_id TEXT NOT NULL,
    awarded_at TEXT NOT NULL,
    PRIMARY KEY (event_id, rule_id)
);

-- The 60-second rule must not depend on the 100-entry visible feed.
CREATE TABLE room_answer_42 (
    room_code TEXT NOT NULL,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE RESTRICT,
    last_at_ns INTEGER NOT NULL,
    PRIMARY KEY (room_code, session_id)
);

CREATE TABLE room_effect_state (
    room_code TEXT PRIMARY KEY,
    last_scene_at_ns INTEGER NOT NULL
);

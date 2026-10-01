ALTER TABLE discovery_progress ADD COLUMN state_json TEXT NOT NULL DEFAULT '';

-- Reconcile the old development checkpoint quietly from stored facts, without
-- evaluating historical source using today's mathematical engine.
UPDATE discovery_progress SET last_sequence = 0, accepted_count = 0;

CREATE TABLE discovery_routes (
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    semantics_version TEXT NOT NULL,
    value TEXT NOT NULL,
    structure TEXT NOT NULL,
    PRIMARY KEY (session_id, semantics_version, value, structure)
);
CREATE TABLE discovery_trig (
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    semantics_version TEXT NOT NULL,
    value TEXT NOT NULL,
    variants INTEGER NOT NULL CHECK (variants BETWEEN 1 AND 3),
    PRIMARY KEY (session_id, semantics_version, value)
);
CREATE TABLE discovery_expressions (
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    semantics_version TEXT NOT NULL,
    expression_identity TEXT NOT NULL,
    first_success_at TEXT NOT NULL,
    PRIMARY KEY (session_id, semantics_version, expression_identity)
);

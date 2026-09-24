-- The owner and triggering action remain linked to committed core history.
-- Nanoseconds keep the 120-second boundary exact across restarts.
CREATE TABLE personal_effects (
    session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE RESTRICT,
    last_calculation_id TEXT NOT NULL,
    last_scene_at_ns INTEGER NOT NULL CHECK (last_scene_at_ns > 0),
    FOREIGN KEY (session_id, last_calculation_id)
        REFERENCES calculations (session_id, id) ON DELETE RESTRICT
);

-- RFC3339Nano text cannot be range-compared lexically when the fractional
-- precision differs. Filter by whole epoch seconds, then check exact times
-- in Go before deciding whether an action is in the 60-second window.
CREATE INDEX calculations_owner_effect_window
    ON calculations (session_id, unixepoch(created_at), seq);

CREATE UNIQUE INDEX calculations_owner_id ON calculations (session_id, id);

CREATE TABLE achievements (
    session_id TEXT NOT NULL,
    achievement_id TEXT NOT NULL,
    calculation_id TEXT NOT NULL,
    earned_at TEXT NOT NULL,
    PRIMARY KEY (session_id, achievement_id),
    FOREIGN KEY (session_id, calculation_id)
        REFERENCES calculations (session_id, id) ON DELETE RESTRICT
);

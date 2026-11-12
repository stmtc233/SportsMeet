CREATE TABLE combined_events (
 id TEXT PRIMARY KEY,
 meet_id TEXT NOT NULL REFERENCES meets(id) ON DELETE CASCADE,
 name TEXT NOT NULL,
 gender TEXT NOT NULL,
 group_label TEXT NOT NULL DEFAULT '',
 note TEXT NOT NULL DEFAULT '',
 rule_version INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX idx_combined_meet ON combined_events(meet_id);
ALTER TABLE events ADD COLUMN combined_id TEXT REFERENCES combined_events(id) ON DELETE CASCADE;
CREATE TABLE combined_components (
 combined_id TEXT NOT NULL REFERENCES combined_events(id) ON DELETE CASCADE,
 event_id TEXT NOT NULL UNIQUE REFERENCES events(id) ON DELETE CASCADE,
 ordinal INTEGER NOT NULL,
 rule_json TEXT NOT NULL,
 PRIMARY KEY(combined_id, ordinal)
);
CREATE TABLE combined_entries (
 combined_id TEXT NOT NULL REFERENCES combined_events(id) ON DELETE CASCADE,
 athlete_id TEXT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
 PRIMARY KEY(combined_id, athlete_id)
);

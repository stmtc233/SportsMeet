CREATE TABLE meet_groups (
 id TEXT PRIMARY KEY,
 meet_id TEXT NOT NULL REFERENCES meets(id) ON DELETE CASCADE,
 name TEXT NOT NULL,
 name_key TEXT NOT NULL,
 ordinal INTEGER NOT NULL
);
CREATE INDEX idx_meet_groups_meet ON meet_groups(meet_id);
CREATE INDEX idx_meet_groups_name ON meet_groups(meet_id, name_key);
ALTER TABLE events ADD COLUMN group_id TEXT REFERENCES meet_groups(id);
CREATE INDEX idx_events_group ON events(group_id);
ALTER TABLE combined_events ADD COLUMN group_id TEXT REFERENCES meet_groups(id);
CREATE INDEX idx_combined_group ON combined_events(group_id);
ALTER TABLE athletes ADD COLUMN group_id TEXT REFERENCES meet_groups(id);
CREATE INDEX idx_athletes_group ON athletes(group_id);
CREATE TABLE unit_groups (
 unit_id TEXT NOT NULL REFERENCES units(id) ON DELETE CASCADE,
 group_id TEXT NOT NULL REFERENCES meet_groups(id),
 PRIMARY KEY(unit_id, group_id)
);
CREATE INDEX idx_unit_groups_group ON unit_groups(group_id);

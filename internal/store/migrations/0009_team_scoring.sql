CREATE TABLE meet_team_scoring (
  meet_id TEXT PRIMARY KEY REFERENCES meets(id) ON DELETE CASCADE,
  rules_json TEXT NOT NULL
);

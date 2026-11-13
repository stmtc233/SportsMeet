-- 当前完整结构快照,与 migrations/ 按序执行后的结果一致;改迁移时同步更新。
-- 赛事
CREATE TABLE meets (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    start_date TEXT NOT NULL,
    end_date TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
);

-- 参赛单位
CREATE TABLE units (
    id TEXT PRIMARY KEY,
    meet_id TEXT NOT NULL REFERENCES meets(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    short_name TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_units_meet ON units(meet_id);

-- 运动员
CREATE TABLE athletes (
 group_id TEXT REFERENCES meet_groups(id),
    id TEXT PRIMARY KEY,
    meet_id TEXT NOT NULL REFERENCES meets(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    gender TEXT NOT NULL DEFAULT '',
    bib TEXT NOT NULL DEFAULT '',
    student_number TEXT NOT NULL DEFAULT '',
    unit_id TEXT REFERENCES units(id) ON DELETE SET NULL,
    class_name TEXT NOT NULL DEFAULT '',
    grade TEXT NOT NULL DEFAULT '',
    birth_date TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    bib_key TEXT NOT NULL DEFAULT '',
    student_number_key TEXT NOT NULL DEFAULT '',
    name_key TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_athletes_meet ON athletes(meet_id);
CREATE INDEX idx_athletes_bib_key ON athletes(meet_id, bib_key);
CREATE INDEX idx_athletes_student_key ON athletes(meet_id, student_number_key);
CREATE INDEX idx_athletes_name_key ON athletes(meet_id, name_key);

-- 项目:一个单项
CREATE TABLE events (
 group_id TEXT REFERENCES meet_groups(id),
    id TEXT PRIMARY KEY,
    meet_id TEXT NOT NULL REFERENCES meets(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    gender TEXT NOT NULL,
    discipline TEXT NOT NULL,
    group_label TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL,
    date TEXT NOT NULL DEFAULT '',
    lane_count INTEGER NOT NULL DEFAULT 8,
    attempt_count INTEGER NOT NULL DEFAULT 3,
    note TEXT NOT NULL DEFAULT '',
    start_mode TEXT NOT NULL DEFAULT '',
    distance_mode TEXT NOT NULL DEFAULT '',
    relay_leg_count INTEGER NOT NULL DEFAULT 0,
    config_json TEXT NOT NULL DEFAULT '{}',
    combined_id TEXT REFERENCES combined_events(id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL
);
CREATE INDEX idx_events_meet ON events(meet_id);
CREATE INDEX idx_events_combined ON events(combined_id);

-- 接力队
CREATE TABLE teams (
    id TEXT PRIMARY KEY,
    event_id TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    unit_id TEXT REFERENCES units(id) ON DELETE SET NULL
);
CREATE INDEX idx_teams_event ON teams(event_id);

CREATE TABLE team_members (
    team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    leg INTEGER NOT NULL,
    athlete_id TEXT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    PRIMARY KEY (team_id, leg)
);
CREATE INDEX idx_team_members_athlete ON team_members(athlete_id);

-- 报名:个人项目用 athlete_id,接力用 team_id
CREATE TABLE entries (
    event_id TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    athlete_id TEXT REFERENCES athletes(id) ON DELETE CASCADE,
    team_id TEXT REFERENCES teams(id) ON DELETE CASCADE,
    seed_mark INTEGER,
    CHECK ((athlete_id IS NULL) != (team_id IS NULL))
);
CREATE UNIQUE INDEX idx_entries_athlete ON entries(event_id, athlete_id) WHERE athlete_id IS NOT NULL;
CREATE UNIQUE INDEX idx_entries_team ON entries(event_id, team_id) WHERE team_id IS NOT NULL;
CREATE INDEX idx_entries_athlete_any ON entries(athlete_id);
CREATE INDEX idx_entries_team_any ON entries(team_id);
CREATE INDEX idx_entries_event ON entries(event_id);

-- 轮次
CREATE TABLE rounds (
    id TEXT PRIMARY KEY,
    event_id TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    stage TEXT NOT NULL,
    ordinal INTEGER NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    advance_by_place INTEGER,
    advance_by_mark INTEGER,
    scheduled_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_rounds_event ON rounds(event_id, ordinal);

-- 组;name 只存自定义名称,started_at 是实际开赛时间
CREATE TABLE heats (
    id TEXT PRIMARY KEY,
    round_id TEXT NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    scheduled_at TEXT NOT NULL DEFAULT '',
    wind INTEGER,
    duration_minutes INTEGER,
    started_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_heats_round ON heats(round_id, ordinal);

-- 位置:position 是实际起跑道次或顺序
CREATE TABLE slots (
    heat_id TEXT NOT NULL REFERENCES heats(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    athlete_id TEXT REFERENCES athletes(id) ON DELETE SET NULL,
    team_id TEXT REFERENCES teams(id) ON DELETE SET NULL,
    PRIMARY KEY (heat_id, position)
);
CREATE INDEX idx_slots_athlete ON slots(athlete_id);
CREATE INDEX idx_slots_team ON slots(team_id);

-- 成绩:mark 径赛毫秒、田赛毫米;detail_json 存远度逐次与高度逐高度记录
CREATE TABLE results (
    heat_id TEXT NOT NULL,
    position INTEGER NOT NULL,
    status TEXT NOT NULL,
    mark INTEGER,
    rank INTEGER,
    qualify TEXT NOT NULL DEFAULT '',
    wind INTEGER,
    detail_json TEXT NOT NULL DEFAULT '{}',
    note TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (heat_id, position),
    FOREIGN KEY (heat_id, position) REFERENCES slots(heat_id, position) ON DELETE CASCADE
);

-- 操作日志,只追加;source: ui | mcp | system
CREATE TABLE audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ts INTEGER NOT NULL,
    source TEXT NOT NULL,
    action TEXT NOT NULL,
    meet_id TEXT NOT NULL DEFAULT '',
    event_id TEXT NOT NULL DEFAULT '',
    heat_id TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    detail_json TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX idx_audit_ts ON audit_log(ts);
CREATE INDEX idx_audit_meet ON audit_log(meet_id, ts);

CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE combined_events (
 group_id TEXT REFERENCES meet_groups(id),
 id TEXT PRIMARY KEY,
 meet_id TEXT NOT NULL REFERENCES meets(id) ON DELETE CASCADE,
 name TEXT NOT NULL,
 gender TEXT NOT NULL,
 group_label TEXT NOT NULL DEFAULT '',
 note TEXT NOT NULL DEFAULT '',
 rule_version INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX idx_combined_meet ON combined_events(meet_id);
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
CREATE INDEX idx_combined_entries_athlete ON combined_entries(athlete_id);

CREATE TABLE meet_team_scoring (
  meet_id TEXT PRIMARY KEY REFERENCES meets(id) ON DELETE CASCADE,
  rules_json TEXT NOT NULL
);

CREATE TABLE meet_groups (
 id TEXT PRIMARY KEY,
 meet_id TEXT NOT NULL REFERENCES meets(id) ON DELETE CASCADE,
 name TEXT NOT NULL,
 name_key TEXT NOT NULL,
 ordinal INTEGER NOT NULL
);
CREATE INDEX idx_meet_groups_meet ON meet_groups(meet_id);
CREATE INDEX idx_meet_groups_name ON meet_groups(meet_id, name_key);
CREATE INDEX idx_events_group ON events(group_id);
CREATE INDEX idx_combined_group ON combined_events(group_id);
CREATE INDEX idx_athletes_group ON athletes(group_id);
CREATE TABLE unit_groups (
 unit_id TEXT NOT NULL REFERENCES units(id) ON DELETE CASCADE,
 group_id TEXT NOT NULL REFERENCES meet_groups(id),
 PRIMARY KEY(unit_id, group_id)
);
CREATE INDEX idx_unit_groups_group ON unit_groups(group_id);

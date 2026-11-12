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
    created_at INTEGER NOT NULL
);
CREATE INDEX idx_athletes_meet ON athletes(meet_id);

-- 项目:一个单项
CREATE TABLE events (
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
    created_at INTEGER NOT NULL
);
CREATE INDEX idx_events_meet ON events(meet_id);

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

-- 轮次
CREATE TABLE rounds (
    id TEXT PRIMARY KEY,
    event_id TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    stage TEXT NOT NULL,
    ordinal INTEGER NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    advance_by_place INTEGER,
    advance_by_mark INTEGER
);
CREATE INDEX idx_rounds_event ON rounds(event_id, ordinal);

-- 组;name 只存自定义名称
CREATE TABLE heats (
    id TEXT PRIMARY KEY,
    round_id TEXT NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    scheduled_at TEXT NOT NULL DEFAULT '',
    wind INTEGER
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

CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

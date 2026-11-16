-- 打印样式:软件全局,跨赛事选用。图片、版本快照随样式删除;前置页与赛事图片属于赛事。
-- 外键列 style_id / meet_id 都是下面主键或唯一约束的第一列,按它们查找与级联删除都走索引。
CREATE TABLE print_styles (
 id TEXT PRIMARY KEY,
 name TEXT NOT NULL UNIQUE,
 params_json TEXT NOT NULL DEFAULT '{}',
 css TEXT NOT NULL DEFAULT '',
 slots_json TEXT NOT NULL DEFAULT '{}',
 templates_json TEXT NOT NULL DEFAULT '{}',
 data_version INTEGER NOT NULL DEFAULT 0,
 version INTEGER NOT NULL DEFAULT 1,
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL
);
CREATE TABLE print_style_versions (
 style_id TEXT NOT NULL REFERENCES print_styles(id) ON DELETE CASCADE,
 version INTEGER NOT NULL,
 source TEXT NOT NULL,
 snapshot_json TEXT NOT NULL,
 created_at INTEGER NOT NULL,
 PRIMARY KEY(style_id, version)
);
CREATE TABLE print_style_assets (
 id TEXT PRIMARY KEY,
 style_id TEXT NOT NULL REFERENCES print_styles(id) ON DELETE CASCADE,
 name TEXT NOT NULL,
 mime TEXT NOT NULL,
 data BLOB NOT NULL,
 sha256 TEXT NOT NULL,
 UNIQUE(style_id, name)
);
CREATE TABLE meet_front_pages (
 id TEXT PRIMARY KEY,
 meet_id TEXT NOT NULL REFERENCES meets(id) ON DELETE CASCADE,
 position INTEGER NOT NULL,
 title TEXT NOT NULL,
 body_md TEXT NOT NULL DEFAULT '',
 reports_json TEXT NOT NULL DEFAULT '[]',
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL
);
CREATE INDEX idx_meet_front_pages_meet ON meet_front_pages(meet_id, position);
CREATE TABLE meet_assets (
 id TEXT PRIMARY KEY,
 meet_id TEXT NOT NULL REFERENCES meets(id) ON DELETE CASCADE,
 name TEXT NOT NULL,
 mime TEXT NOT NULL,
 data BLOB NOT NULL,
 sha256 TEXT NOT NULL,
 UNIQUE(meet_id, name)
);

-- 组的实际开赛时间(计划时间格式,空 = 未记录);已有组保持未记录。
ALTER TABLE heats ADD COLUMN started_at TEXT NOT NULL DEFAULT '';

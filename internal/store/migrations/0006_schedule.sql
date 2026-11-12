-- 轮次开始时间(计划时间格式,空 = 未安排)与组的预计用时(分钟,NULL = 未填);已有轮次与组保持未安排。
ALTER TABLE rounds ADD COLUMN scheduled_at TEXT NOT NULL DEFAULT '';
ALTER TABLE heats ADD COLUMN duration_minutes INTEGER;
-- 操作日志按赛事筛选时按时刻倒序读取。
CREATE INDEX idx_audit_meet ON audit_log(meet_id, ts);

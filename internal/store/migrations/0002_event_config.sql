-- 缺省值保留已有项目的道次、试跳与自定义棒次。
ALTER TABLE events ADD COLUMN start_mode TEXT NOT NULL DEFAULT '';
ALTER TABLE events ADD COLUMN distance_mode TEXT NOT NULL DEFAULT '';
ALTER TABLE events ADD COLUMN relay_leg_count INTEGER NOT NULL DEFAULT 0;

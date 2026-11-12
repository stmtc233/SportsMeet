-- 外键所在列的索引:删除运动员、队伍、全能项目时按这些列查找受影响的行
CREATE INDEX idx_slots_team ON slots(team_id);
CREATE INDEX idx_entries_team_any ON entries(team_id);
CREATE INDEX idx_team_members_athlete ON team_members(athlete_id);
CREATE INDEX idx_events_combined ON events(combined_id);
CREATE INDEX idx_combined_entries_athlete ON combined_entries(athlete_id);

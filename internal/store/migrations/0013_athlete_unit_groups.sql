-- Every athlete with a unit is in one of the unit's groups, and in the only
-- one when the unit has one. A unit lacking the group of one of its
-- athletes joins it, so no athlete drops out of its unit in the 报名 tree;
-- then the athletes of no group in a unit of one group go into it.
INSERT OR IGNORE INTO unit_groups(unit_id, group_id)
SELECT DISTINCT unit_id, group_id FROM athletes WHERE unit_id IS NOT NULL AND group_id IS NOT NULL;
UPDATE athletes SET group_id = (SELECT ug.group_id FROM unit_groups ug WHERE ug.unit_id = athletes.unit_id)
WHERE group_id IS NULL AND unit_id IN (SELECT unit_id FROM unit_groups GROUP BY unit_id HAVING COUNT(*) = 1);

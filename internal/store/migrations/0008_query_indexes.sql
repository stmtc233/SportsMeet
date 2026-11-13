-- Canonical keys are maintained by service; indexes are deliberately nonunique.
ALTER TABLE athletes ADD COLUMN bib_key TEXT NOT NULL DEFAULT '';
ALTER TABLE athletes ADD COLUMN student_number_key TEXT NOT NULL DEFAULT '';
ALTER TABLE athletes ADD COLUMN name_key TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_athletes_bib_key ON athletes(meet_id, bib_key);
CREATE INDEX idx_athletes_student_key ON athletes(meet_id, student_number_key);
CREATE INDEX idx_athletes_name_key ON athletes(meet_id, name_key);
-- Participant-specific partial indexes do not cover an entire event's entries.
CREATE INDEX idx_entries_event ON entries(event_id);

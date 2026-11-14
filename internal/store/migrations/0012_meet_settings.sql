-- Each meet keeps its own arrange and print settings, starting from the
-- app-wide preferences they were before.
ALTER TABLE meets ADD COLUMN settings TEXT NOT NULL DEFAULT '';
UPDATE meets SET settings = COALESCE((SELECT value FROM settings WHERE key = 'preferences'), '');

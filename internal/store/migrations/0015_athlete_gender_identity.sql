-- Historical gender is the competition category; identity is never inferred.
ALTER TABLE athletes RENAME COLUMN gender TO competition_gender;
ALTER TABLE athletes ADD COLUMN gender_identity TEXT NOT NULL DEFAULT '';

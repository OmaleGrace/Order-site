-- Saved delivery details on the customer profile (edited under Account > Edit profile).
-- Idempotent, so it is safe whether or not the columns already exist.
ALTER TABLE users ADD COLUMN IF NOT EXISTS address_line TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS delivery_note TEXT NOT NULL DEFAULT '';

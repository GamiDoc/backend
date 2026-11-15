ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;

-- Accounts created before email verification existed stay usable.
UPDATE users SET email_verified_at = created_at WHERE email_verified_at IS NULL;

-- 000018_token_version: add token_version to users for JWT revocation on password reset
ALTER TABLE users ADD COLUMN IF NOT EXISTS token_version INT NOT NULL DEFAULT 0;

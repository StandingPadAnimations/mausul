CREATE TABLE IF NOT EXISTS tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    realm TEXT NOT NULL UNIQUE,
    access_token TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE webmentions ADD COLUMN is_private INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_webmentions_target_private
ON webmentions (target, is_private);

CREATE TABLE IF NOT EXISTS sessions (
    sid_hash TEXT PRIMARY KEY NOT NULL,
    data TEXT NOT NULL,
    expires_at INTEGER NOT NULL
);

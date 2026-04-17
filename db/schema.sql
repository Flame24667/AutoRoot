-- SQLite schema for smartphone model mapping
CREATE TABLE IF NOT EXISTS models (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    vendor TEXT NOT NULL,
    product TEXT NOT NULL,
    model TEXT NOT NULL,
    android_version TEXT,
    os_version TEXT,
    protocol TEXT,
    firmware_path TEXT
);

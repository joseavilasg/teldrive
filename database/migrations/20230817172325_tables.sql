-- +goose Up

CREATE TABLE drive.files (
    id TEXT PRIMARY KEY NOT NULL DEFAULT drive.generate_uid(16),
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    path TEXT,
    size BIGINT,
    starred BOOLEAN NOT NULL,
    depth INTEGER,
    user_id INTEGER NOT NULL,
    parent_id TEXT,
    status TEXT DEFAULT 'active'::TEXT,
    parts JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT timezone('utc'::TEXT, now()),
    updated_at TIMESTAMP NOT NULL DEFAULT timezone('utc'::TEXT, now())
);

CREATE TABLE drive.uploads (
    upload_id TEXT NOT NULL,
    name TEXT NOT NULL,
    part_no INTEGER NOT NULL,
    size BIGINT NOT NULL,
    url TEXT NOT NULL,
	user_id INTEGER NOT NULL,
    created_at TIMESTAMP NULL DEFAULT timezone('utc'::TEXT, now())
);

CREATE TABLE drive.users (
    id SERIAL PRIMARY KEY,
    full_name TEXT,
    user_name TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT timezone('utc'::TEXT, now()),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT timezone('utc'::TEXT, now())
);

CREATE INDEX name_search_idx ON drive.files USING gin (drive.get_tsvector(name), updated_at);

CREATE INDEX name_numeric_idx ON drive.files (name COLLATE numeric NULLS FIRST, updated_at DESC);

CREATE INDEX path_idx ON drive.files (path, user_id);

CREATE INDEX parent_idx ON drive.files (parent_id, user_id);

CREATE INDEX starred_updated_at_idx ON drive.files (starred, updated_at DESC);

CREATE INDEX status_idx ON drive.files (status, user_id);

CREATE INDEX user_id_idx ON drive.files (user_id);

CREATE UNIQUE INDEX unique_file ON drive.files (name, parent_id, user_id) WHERE (status= 'active');

-- +goose Down
DROP TABLE IF EXISTS drive.files;
DROP TABLE IF EXISTS drive.uploads;
DROP TABLE IF EXISTS drive.users;
DROP INDEX IF EXISTS drive.name_search_idx;
DROP INDEX IF EXISTS drive.name_numeric_idx;
DROP INDEX IF EXISTS drive.path_idx;
DROP INDEX IF EXISTS drive.parent_idx;
DROP INDEX IF EXISTS drive.starred_updated_at_idx;
DROP INDEX IF EXISTS drive.status_idx;
DROP INDEX IF EXISTS drive.user_id_idx;

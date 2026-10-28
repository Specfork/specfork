CREATE TABLE users (
    github_id    BIGINT PRIMARY KEY,
    login        TEXT NOT NULL,
    signed_in_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE scores (
    github_id  BIGINT NOT NULL REFERENCES users (github_id) ON DELETE CASCADE,
    spec       TEXT NOT NULL,
    score      SMALLINT NOT NULL CHECK (score BETWEEN 1 AND 5),
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (github_id, spec)
);

CREATE TABLE counters (
    day   TEXT NOT NULL,
    tool  TEXT NOT NULL,
    spec  TEXT NOT NULL,
    count BIGINT NOT NULL,
    PRIMARY KEY (day, tool, spec)
);

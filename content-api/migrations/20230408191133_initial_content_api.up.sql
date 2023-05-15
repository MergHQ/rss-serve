CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE user_feeds (
  feed_id UUID NOT NULL,
  user_id UUID NOT NULL
);

CREATE TABLE feed_content (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  feed_id    UUID NOT NULL,
  title      TEXT NOT NULL,
  img_url    TEXT,
  "link"     TEXT UNIQUE NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

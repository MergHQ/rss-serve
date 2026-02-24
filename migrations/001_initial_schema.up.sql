CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "citext";

-- Users table from usr-api
CREATE TABLE users (
  id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Feeds and user_feeds tables from feed-api
CREATE TABLE feeds (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  url        CITEXT NOT NULL UNIQUE,
  title      CITEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE user_feeds (
  user_id UUID NOT NULL,
  feed_id UUID NOT NULL,

  CONSTRAINT fk_feed FOREIGN KEY (feed_id) REFERENCES feeds (id)
);

-- Feed content table from content-api
CREATE TABLE feed_content (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  feed_id    UUID NOT NULL,
  "guid"     TEXT UNIQUE NOT NULL, 
  title      TEXT NOT NULL,
  img_url    TEXT,
  "link"     TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "citext";

CREATE TABLE feeds (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  url        CITEXT NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE user_feeds (
  user_id UUID NOT NULL,
  feed_id UUID NOT NULL,

  CONSTRAINT fk_feed FOREIGN KEY (feed_id) REFERENCES feeds (id)
);


-- Create tags table for user-specific tags
CREATE TABLE tags (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id    UUID NOT NULL,
  name       CITEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users (id),
  CONSTRAINT unique_user_tag_name UNIQUE (user_id, name)
);

-- Create feed_tags junction table for many-to-many relationship
CREATE TABLE feed_tags (
  feed_id    UUID NOT NULL,
  tag_id     UUID NOT NULL,
  CONSTRAINT fk_feed FOREIGN KEY (feed_id) REFERENCES feeds (id),
  CONSTRAINT fk_tag FOREIGN KEY (tag_id) REFERENCES tags (id),
  CONSTRAINT unique_feed_tag UNIQUE (feed_id, tag_id)
);

-- Create index for faster tag-based content filtering
CREATE INDEX idx_feed_tags_feed_id ON feed_tags(feed_id);
CREATE INDEX idx_feed_tags_tag_id ON feed_tags(tag_id);
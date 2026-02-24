-- Add published_at column to feed_content table
ALTER TABLE feed_content ADD COLUMN published_at TIMESTAMPTZ;

-- Update existing records to use created_at as published_at initially
UPDATE feed_content SET published_at = created_at;

-- Create a function to update published_at when inserting new records
CREATE OR REPLACE FUNCTION set_published_at() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.published_at IS NULL THEN
        NEW.published_at = NEW.created_at;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger to ensure published_at is set
CREATE TRIGGER ensure_published_at
BEFORE INSERT OR UPDATE ON feed_content
FOR EACH ROW EXECUTE FUNCTION set_published_at();
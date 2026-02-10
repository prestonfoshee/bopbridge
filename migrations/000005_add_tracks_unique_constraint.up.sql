-- Add unique constraint on user_id and spotify_track_id for ON CONFLICT upserts
-- The existing partial unique index (WHERE deleted_at IS NULL) doesn't work with ON CONFLICT

-- First, drop the partial unique index
DROP INDEX IF EXISTS idx_user_spotify_track;

-- Create a proper unique constraint (not a partial index)
ALTER TABLE tracks ADD CONSTRAINT uq_user_spotify_track UNIQUE (user_id, spotify_track_id);

-- Re-create a regular index for soft-deleted filtering if needed
CREATE INDEX IF NOT EXISTS idx_tracks_user_spotify_deleted ON tracks(user_id, spotify_track_id, deleted_at);

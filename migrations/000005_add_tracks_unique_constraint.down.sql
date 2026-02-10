-- Revert: remove the unique constraint and restore the partial unique index

-- Drop the index we created
DROP INDEX IF EXISTS idx_tracks_user_spotify_deleted;

-- Drop the unique constraint
ALTER TABLE tracks DROP CONSTRAINT IF EXISTS uq_user_spotify_track;

-- Restore the original partial unique index
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_spotify_track ON tracks(user_id, spotify_track_id) WHERE deleted_at IS NULL;

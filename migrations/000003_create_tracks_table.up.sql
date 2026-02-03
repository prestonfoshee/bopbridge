-- Create tracks table
CREATE TABLE IF NOT EXISTS tracks (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    spotify_track_id VARCHAR(255) NOT NULL,
    name VARCHAR(500) NOT NULL,
    artist_name VARCHAR(500) NOT NULL,
    album_name VARCHAR(500),
    genres JSONB,
    audio_features JSONB,
    is_liked BOOLEAN DEFAULT TRUE,
    added_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Create indexes for faster lookups
CREATE INDEX IF NOT EXISTS idx_tracks_user_id ON tracks(user_id);
CREATE INDEX IF NOT EXISTS idx_tracks_artist_name ON tracks(artist_name);
CREATE INDEX IF NOT EXISTS idx_tracks_is_liked ON tracks(is_liked);
CREATE INDEX IF NOT EXISTS idx_tracks_deleted_at ON tracks(deleted_at);

-- Create unique composite index to prevent duplicate tracks per user
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_spotify_track ON tracks(user_id, spotify_track_id) WHERE deleted_at IS NULL;

-- Add comment to table
COMMENT ON TABLE tracks IS 'Spotify tracks liked by users with audio features for analysis';
COMMENT ON COLUMN tracks.audio_features IS 'JSON object containing Spotify audio analysis data (acousticness, danceability, energy, etc.)';
COMMENT ON COLUMN tracks.genres IS 'Array of genre strings associated with the track/artist';

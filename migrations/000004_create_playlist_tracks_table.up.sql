-- Create playlist_tracks junction table
CREATE TABLE IF NOT EXISTS playlist_tracks (
    id SERIAL PRIMARY KEY,
    playlist_id INTEGER NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    track_id INTEGER NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for faster lookups
CREATE INDEX IF NOT EXISTS idx_playlist_tracks_playlist_id ON playlist_tracks(playlist_id);
CREATE INDEX IF NOT EXISTS idx_playlist_tracks_track_id ON playlist_tracks(track_id);

-- Create unique composite index to prevent duplicate tracks in same playlist
CREATE UNIQUE INDEX IF NOT EXISTS idx_playlist_track ON playlist_tracks(playlist_id, track_id);

-- Create index for ordering tracks by position
CREATE INDEX IF NOT EXISTS idx_playlist_tracks_position ON playlist_tracks(playlist_id, position);

-- Add comment to table
COMMENT ON TABLE playlist_tracks IS 'Many-to-many relationship between playlists and tracks with ordering';
COMMENT ON COLUMN playlist_tracks.position IS 'Order of track in playlist (0-based index)';

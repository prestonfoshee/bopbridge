-- Create playlists table
CREATE TABLE IF NOT EXISTS playlists (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    spotify_playlist_id VARCHAR(255),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    strategy VARCHAR(50) NOT NULL,
    params JSONB,
    track_count INTEGER DEFAULT 0,
    is_public BOOLEAN DEFAULT FALSE,
    is_synced BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for faster lookups
CREATE INDEX IF NOT EXISTS idx_playlists_user_id ON playlists(user_id);
CREATE INDEX IF NOT EXISTS idx_playlists_spotify_playlist_id ON playlists(spotify_playlist_id);
CREATE INDEX IF NOT EXISTS idx_playlists_strategy ON playlists(strategy);
CREATE INDEX IF NOT EXISTS idx_playlists_is_synced ON playlists(is_synced);

-- Add comment to table
COMMENT ON TABLE playlists IS 'Generated playlists that may or may not be synced to Spotify';
COMMENT ON COLUMN playlists.strategy IS 'Algorithm used to generate playlist: mood, genre, energy, similarity, discovery';
COMMENT ON COLUMN playlists.params IS 'JSON parameters used for playlist generation';

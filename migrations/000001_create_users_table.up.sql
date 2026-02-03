-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    spotify_id VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255),
    display_name VARCHAR(255) NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_expiry TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index on spotify_id for faster lookups
CREATE INDEX IF NOT EXISTS idx_users_spotify_id ON users(spotify_id);

-- Add comment to table
COMMENT ON TABLE users IS 'Users who have authenticated via Spotify OAuth';
COMMENT ON COLUMN users.access_token IS 'Spotify OAuth access token (should be encrypted in production)';
COMMENT ON COLUMN users.refresh_token IS 'Spotify OAuth refresh token (should be encrypted in production)';

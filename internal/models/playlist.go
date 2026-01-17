package models

import (
	"time"

	"gorm.io/gorm"
)

// PlaylistStrategy defines the algorithm used to generate a playlist
type PlaylistStrategy string

const (
	StrategyMood       PlaylistStrategy = "mood"       // Based on valence/energy audio features
	StrategyGenre      PlaylistStrategy = "genre"      // Grouped by genre
	StrategyEnergy     PlaylistStrategy = "energy"     // High/low energy playlists
	StrategySimilarity PlaylistStrategy = "similarity" // Similar to seed tracks
	StrategyDiscovery  PlaylistStrategy = "discovery"  // Mix of familiar + new recommendations
)

// Playlist represents a generated playlist that may or may not be synced to Spotify.
// It tracks the generation strategy and parameters used to create it.
type Playlist struct {
	ID                uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID            uint             `gorm:"index;not null" json:"user_id"`
	User              User             `gorm:"foreignKey:UserID" json:"-"`                  // Belongs to User
	SpotifyPlaylistID string           `gorm:"size:255" json:"spotify_playlist_id,omitempty"` // Set after syncing to Spotify
	Name              string           `gorm:"size:255;not null" json:"name"`
	Description       string           `gorm:"type:text" json:"description,omitempty"`
	Strategy          PlaylistStrategy `gorm:"size:50;not null" json:"strategy"`
	Params            string           `gorm:"type:jsonb" json:"params,omitempty"` // JSON params used for generation
	TrackCount        int              `gorm:"default:0" json:"track_count"`
	IsPublic          bool             `gorm:"default:false" json:"is_public"`    // Visibility on Spotify
	IsSynced          bool             `gorm:"default:false" json:"is_synced"`    // Whether it's been pushed to Spotify
	CreatedAt         time.Time        `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time        `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName overrides the default table name
func (Playlist) TableName() string {
	return "playlists"
}

// BeforeCreate hook to validate strategy before saving
func (p *Playlist) BeforeCreate(tx *gorm.DB) error {
	// Validate strategy is one of the allowed values
	switch p.Strategy {
	case StrategyMood, StrategyGenre, StrategyEnergy, StrategySimilarity, StrategyDiscovery:
		return nil
	default:
		p.Strategy = StrategyMood // Default to mood if invalid
	}
	return nil
}

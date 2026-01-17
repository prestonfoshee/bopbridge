package models

import "time"

// User represents a user who has authenticated via Spotify OAuth.
// Tokens are stored encrypted in the database and hidden from JSON responses.
type User struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SpotifyID    string    `gorm:"uniqueIndex;size:255;not null" json:"spotify_id"`
	Email        string    `gorm:"size:255" json:"email,omitempty"` // Optional: Spotify may not always provide email
	DisplayName  string    `gorm:"size:255;not null" json:"display_name"`
	AccessToken  string    `gorm:"type:text;not null" json:"-"` // Hidden from JSON, should be encrypted
	RefreshToken string    `gorm:"type:text;not null" json:"-"` // Hidden from JSON, should be encrypted
	TokenExpiry  time.Time `gorm:"not null" json:"-"`           // Hidden: internal token management
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName overrides the default table name
func (User) TableName() string {
	return "users"
}
package models

import (
	"time"

	"gorm.io/gorm"
)

// Track represents a Spotify track that a user has liked or added to their library.
// It stores basic track metadata along with audio features for analysis.
type Track struct {
	ID             uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         uint           `gorm:"index;not null" json:"user_id"`
	User           User           `gorm:"foreignKey:UserID" json:"-"`
	SpotifyTrackID string         `gorm:"size:255;not null;uniqueIndex:idx_user_spotify_track" json:"spotify_track_id"`
	Name           string         `gorm:"size:500;not null" json:"name"`
	ArtistName     string         `gorm:"size:500;not null;index" json:"artist_name"`
	AlbumName      string         `gorm:"size:500" json:"album_name"`
	Genres         []string       `gorm:"type:jsonb" json:"genres,omitempty"`
	AudioFeatures  *AudioFeatures `gorm:"type:jsonb" json:"audio_features,omitempty"`
	IsLiked        bool           `gorm:"default:true;index" json:"is_liked"`
	AddedAt        time.Time      `gorm:"not null" json:"added_at"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the default table name
func (Track) TableName() string {
	return "tracks"
}

// AudioFeatures represents the audio analysis data provided by Spotify.
// These features can be used to analyze music characteristics and generate
// playlists based on mood, energy, danceability, etc.
type AudioFeatures struct {
	Acousticness     float64 `json:"acousticness"`     // 0.0 to 1.0: confidence the track is acoustic
	Danceability     float64 `json:"danceability"`     // 0.0 to 1.0: how suitable for dancing
	Energy           float64 `json:"energy"`           // 0.0 to 1.0: intensity and activity
	Instrumentalness float64 `json:"instrumentalness"` // 0.0 to 1.0: predicts if track has no vocals
	Liveness         float64 `json:"liveness"`         // 0.0 to 1.0: presence of audience in recording
	Loudness         float64 `json:"loudness"`         // -60 to 0 dB: overall loudness
	Speechiness      float64 `json:"speechiness"`      // 0.0 to 1.0: presence of spoken words
	Valence          float64 `json:"valence"`          // 0.0 to 1.0: musical positiveness (happiness)
	Tempo            float64 `json:"tempo"`            // BPM: overall estimated tempo
	Key              int     `json:"key"`              // 0-11: pitch class notation (0 = C, 1 = C#, etc.)
	Mode             int     `json:"mode"`             // 0 = minor, 1 = major
	TimeSignature    int     `json:"time_signature"`   // 3-7: estimated time signature
	DurationMs       int     `json:"duration_ms"`      // Duration in milliseconds
}

// PlaylistTrack represents the many-to-many relationship between playlists and tracks.
// It maintains the order of tracks within a playlist.
type PlaylistTrack struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	PlaylistID uint      `gorm:"not null;index;uniqueIndex:idx_playlist_track" json:"playlist_id"`
	Playlist   Playlist  `gorm:"foreignKey:PlaylistID" json:"-"`
	TrackID    uint      `gorm:"not null;index;uniqueIndex:idx_playlist_track" json:"track_id"`
	Track      Track     `gorm:"foreignKey:TrackID" json:"-"`
	Position   int       `gorm:"not null" json:"position"` // Order of track in playlist (0-based)
	AddedAt    time.Time `gorm:"autoCreateTime" json:"added_at"`
}

// TableName overrides the default table name
func (PlaylistTrack) TableName() string {
	return "playlist_tracks"
}

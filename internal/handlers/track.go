package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prestonfoshee/bopbridge/internal/middleware"
	"github.com/prestonfoshee/bopbridge/internal/services"
	"github.com/prestonfoshee/bopbridge/internal/utils"
)

// TrackHandler handles track-related HTTP requests.
type TrackHandler struct {
	trackService   services.TrackService
	analyzerService services.AnalyzerService
}

// NewTrackHandler creates a new TrackHandler.
func NewTrackHandler(trackService services.TrackService, analyzerService services.AnalyzerService) *TrackHandler {
	return &TrackHandler{
		trackService:   trackService,
		analyzerService: analyzerService,
	}
}

// SyncTracksHandler syncs all liked songs from Spotify to the database.
// POST /api/v1/tracks/sync
func (h *TrackHandler) SyncTracksHandler(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User ID not found")
		return
	}

	// Sync liked songs from Spotify
	count, err := h.trackService.SyncLikedSongs(c.Request.Context(), userID)
	if err != nil {
		utils.InternalErrorResponse(c, err)
		return
	}

	utils.SuccessMessageResponse(c, http.StatusOK, "Tracks synced successfully", gin.H{
		"synced_count": count,
	})
}

// GetTracksHandler retrieves all tracks for the authenticated user.
// GET /api/v1/tracks
func (h *TrackHandler) GetTracksHandler(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User ID not found")
		return
	}

	tracks, err := h.trackService.GetUserTracks(c.Request.Context(), userID)
	if err != nil {
		utils.InternalErrorResponse(c, err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, gin.H{
		"tracks": tracks,
		"count":  len(tracks),
	})
}

// GetTrackStatsHandler returns statistics about the user's tracks.
// GET /api/v1/tracks/stats
func (h *TrackHandler) GetTrackStatsHandler(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User ID not found")
		return
	}

	// Get all user tracks
	tracks, err := h.trackService.GetUserTracks(c.Request.Context(), userID)
	if err != nil {
		utils.InternalErrorResponse(c, err)
		return
	}

	// Calculate various statistics
	totalTracks, _ := h.trackService.GetTrackCount(c.Request.Context(), userID)
	genreDistribution := h.analyzerService.CalculateGenreDistribution(tracks)
	artistDistribution := h.analyzerService.CalculateArtistDistribution(tracks)
	topArtists := h.analyzerService.GetTopArtists(tracks, 10)
	moodGroups := h.analyzerService.GroupByMood(tracks)

	// Count tracks per mood
	moodCounts := make(map[string]int)
	for mood, moodTracks := range moodGroups {
		moodCounts[mood] = len(moodTracks)
	}

	utils.SuccessResponse(c, http.StatusOK, gin.H{
		"total_tracks":        totalTracks,
		"unique_artists":      len(artistDistribution),
		"unique_genres":       len(genreDistribution),
		"genre_distribution":  genreDistribution,
		"top_artists":         topArtists,
		"mood_distribution":   moodCounts,
	})
}

// GetArtistsHandler returns a list of all unique artists in the user's library.
// GET /api/v1/tracks/artists
func (h *TrackHandler) GetArtistsHandler(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User ID not found")
		return
	}

	artists, err := h.trackService.GetUniqueArtists(c.Request.Context(), userID)
	if err != nil {
		utils.InternalErrorResponse(c, err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, gin.H{
		"artists": artists,
		"count":   len(artists),
	})
}

// GetTracksByArtistHandler returns all tracks by a specific artist.
// GET /api/v1/tracks/artist/:name
func (h *TrackHandler) GetTracksByArtistHandler(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User ID not found")
		return
	}

	artistName := c.Param("name")
	if artistName == "" {
		utils.BadRequestResponse(c, "Artist name is required")
		return
	}

	tracks, err := h.trackService.GetTracksByArtist(c.Request.Context(), userID, artistName)
	if err != nil {
		utils.InternalErrorResponse(c, err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, gin.H{
		"artist": artistName,
		"tracks": tracks,
		"count":  len(tracks),
	})
}

// SyncAudioFeaturesHandler syncs audio features for tracks that don't have them.
// POST /api/v1/tracks/sync-features
func (h *TrackHandler) SyncAudioFeaturesHandler(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User ID not found")
		return
	}

	count, err := h.trackService.SyncAudioFeatures(c.Request.Context(), userID)
	if err != nil {
		utils.InternalErrorResponse(c, err)
		return
	}

	utils.SuccessMessageResponse(c, http.StatusOK, "Audio features synced successfully", gin.H{
		"updated_count": count,
	})
}

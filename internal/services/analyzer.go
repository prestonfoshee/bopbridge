package services

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/prestonfoshee/bopbridge/internal/models"
)

// AnalyzerService defines the interface for analyzing track data.
// It provides methods to group, filter, and analyze tracks based on various criteria.
type AnalyzerService interface {
	// GroupByArtist groups tracks by their artist name.
	GroupByArtist(tracks []models.Track) map[string][]models.Track

	// GroupByGenre groups tracks by their genres (a track can appear in multiple groups).
	GroupByGenre(tracks []models.Track) map[string][]models.Track

	// GroupByMood groups tracks into mood categories based on audio features.
	// Moods include: happy, sad, energetic, calm, etc.
	GroupByMood(tracks []models.Track) map[string][]models.Track

	// FindSimilarTracks finds tracks similar to a given track based on audio features.
	FindSimilarTracks(track models.Track, candidates []models.Track, limit int) []models.Track

	// CalculateGenreDistribution calculates the count of tracks per genre.
	CalculateGenreDistribution(tracks []models.Track) map[string]int

	// CalculateArtistDistribution calculates the count of tracks per artist.
	CalculateArtistDistribution(tracks []models.Track) map[string]int

	// FilterByEnergyRange filters tracks within a specific energy range (0.0 to 1.0).
	FilterByEnergyRange(tracks []models.Track, min, max float64) []models.Track

	// FilterByValenceRange filters tracks within a specific valence (happiness) range (0.0 to 1.0).
	FilterByValenceRange(tracks []models.Track, min, max float64) []models.Track

	// GetTopArtists returns the top N artists by track count.
	GetTopArtists(tracks []models.Track, limit int) []ArtistStats
}

// ArtistStats contains statistics about an artist.
type ArtistStats struct {
	Name       string `json:"name"`
	TrackCount int    `json:"track_count"`
}

// analyzerService is the concrete implementation of AnalyzerService.
type analyzerService struct{}

// NewAnalyzerService creates a new AnalyzerService.
func NewAnalyzerService() AnalyzerService {
	return &analyzerService{}
}

// GroupByArtist groups tracks by their artist name.
func (s *analyzerService) GroupByArtist(tracks []models.Track) map[string][]models.Track {
	grouped := make(map[string][]models.Track)
	for _, track := range tracks {
		artistName := track.ArtistName
		grouped[artistName] = append(grouped[artistName], track)
	}
	return grouped
}

// GroupByGenre groups tracks by their genres.
func (s *analyzerService) GroupByGenre(tracks []models.Track) map[string][]models.Track {
	grouped := make(map[string][]models.Track)
	for _, track := range tracks {
		if len(track.Genres) == 0 {
			// Group tracks without genres under "Unknown"
			grouped["Unknown"] = append(grouped["Unknown"], track)
			continue
		}
		for _, genre := range track.Genres {
			// Normalize genre names (lowercase, trim spaces)
			normalizedGenre := strings.TrimSpace(strings.ToLower(genre))
			grouped[normalizedGenre] = append(grouped[normalizedGenre], track)
		}
	}
	return grouped
}

// GroupByMood groups tracks into mood categories based on audio features.
func (s *analyzerService) GroupByMood(tracks []models.Track) map[string][]models.Track {
	moods := map[string][]models.Track{
		"happy":     {},
		"sad":       {},
		"energetic": {},
		"calm":      {},
		"party":     {},
		"chill":     {},
		"focus":     {},
	}

	for _, track := range tracks {
		if track.AudioFeatures == nil {
			continue // Skip tracks without audio features
		}

		af := track.AudioFeatures

		// Happy: high valence (> 0.6)
		if af.Valence > 0.6 {
			moods["happy"] = append(moods["happy"], track)
		}

		// Sad: low valence (< 0.4)
		if af.Valence < 0.4 {
			moods["sad"] = append(moods["sad"], track)
		}

		// Energetic: high energy (> 0.7)
		if af.Energy > 0.7 {
			moods["energetic"] = append(moods["energetic"], track)
		}

		// Calm: low energy (< 0.4) and low valence
		if af.Energy < 0.4 {
			moods["calm"] = append(moods["calm"], track)
		}

		// Party: high danceability (> 0.7) and high energy
		if af.Danceability > 0.7 && af.Energy > 0.6 {
			moods["party"] = append(moods["party"], track)
		}

		// Chill: low energy and high valence
		if af.Energy < 0.5 && af.Valence > 0.5 {
			moods["chill"] = append(moods["chill"], track)
		}

		// Focus: low speechiness (< 0.3) and medium energy
		if af.Speechiness < 0.3 && af.Energy > 0.3 && af.Energy < 0.7 {
			moods["focus"] = append(moods["focus"], track)
		}
	}

	return moods
}

// FindSimilarTracks finds tracks similar to a given track based on audio features.
func (s *analyzerService) FindSimilarTracks(track models.Track, candidates []models.Track, limit int) []models.Track {
	if track.AudioFeatures == nil {
		return []models.Track{}
	}

	type trackWithDistance struct {
		track    models.Track
		distance float64
	}

	var similarities []trackWithDistance

	for _, candidate := range candidates {
		if candidate.ID == track.ID || candidate.AudioFeatures == nil {
			continue // Skip the same track or tracks without features
		}

		// Calculate Euclidean distance between audio features
		distance := calculateAudioFeatureDistance(track.AudioFeatures, candidate.AudioFeatures)
		similarities = append(similarities, trackWithDistance{
			track:    candidate,
			distance: distance,
		})
	}

	// Sort by distance (ascending - smaller distance = more similar)
	sort.Slice(similarities, func(i, j int) bool {
		return similarities[i].distance < similarities[j].distance
	})

	// Return top N most similar tracks
	result := []models.Track{}
	for i := 0; i < limit && i < len(similarities); i++ {
		result = append(result, similarities[i].track)
	}

	return result
}

// CalculateGenreDistribution calculates the count of tracks per genre.
func (s *analyzerService) CalculateGenreDistribution(tracks []models.Track) map[string]int {
	distribution := make(map[string]int)
	for _, track := range tracks {
		if len(track.Genres) == 0 {
			distribution["Unknown"]++
			continue
		}
		for _, genre := range track.Genres {
			normalizedGenre := strings.TrimSpace(strings.ToLower(genre))
			distribution[normalizedGenre]++
		}
	}
	return distribution
}

// CalculateArtistDistribution calculates the count of tracks per artist.
func (s *analyzerService) CalculateArtistDistribution(tracks []models.Track) map[string]int {
	distribution := make(map[string]int)
	for _, track := range tracks {
		distribution[track.ArtistName]++
	}
	return distribution
}

// FilterByEnergyRange filters tracks within a specific energy range.
func (s *analyzerService) FilterByEnergyRange(tracks []models.Track, min, max float64) []models.Track {
	var filtered []models.Track
	for _, track := range tracks {
		if track.AudioFeatures != nil &&
			track.AudioFeatures.Energy >= min &&
			track.AudioFeatures.Energy <= max {
			filtered = append(filtered, track)
		}
	}
	return filtered
}

// FilterByValenceRange filters tracks within a specific valence range.
func (s *analyzerService) FilterByValenceRange(tracks []models.Track, min, max float64) []models.Track {
	var filtered []models.Track
	for _, track := range tracks {
		if track.AudioFeatures != nil &&
			track.AudioFeatures.Valence >= min &&
			track.AudioFeatures.Valence <= max {
			filtered = append(filtered, track)
		}
	}
	return filtered
}

// GetTopArtists returns the top N artists by track count.
func (s *analyzerService) GetTopArtists(tracks []models.Track, limit int) []ArtistStats {
	distribution := s.CalculateArtistDistribution(tracks)

	// Convert map to slice for sorting
	var stats []ArtistStats
	for name, count := range distribution {
		stats = append(stats, ArtistStats{
			Name:       name,
			TrackCount: count,
		})
	}

	// Sort by track count (descending)
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].TrackCount > stats[j].TrackCount
	})

	// Return top N
	if limit > len(stats) {
		limit = len(stats)
	}
	return stats[:limit]
}

// calculateAudioFeatureDistance calculates the Euclidean distance between two audio feature sets.
// Smaller distance means more similar tracks.
func calculateAudioFeatureDistance(a, b *models.AudioFeatures) float64 {
	if a == nil || b == nil {
		return math.MaxFloat64
	}

	// Normalize tempo to 0-1 range (assuming tempo range 40-200 BPM)
	normA := normalizeAudioFeatures(a)
	normB := normalizeAudioFeatures(b)

	// Calculate squared differences for each feature
	dAcousticness := normA.Acousticness - normB.Acousticness
	dDanceability := normA.Danceability - normB.Danceability
	dEnergy := normA.Energy - normB.Energy
	dInstrumentalness := normA.Instrumentalness - normB.Instrumentalness
	dValence := normA.Valence - normB.Valence
	dTempo := normA.Tempo - normB.Tempo

	// Euclidean distance
	distance := math.Sqrt(
		dAcousticness*dAcousticness +
			dDanceability*dDanceability +
			dEnergy*dEnergy +
			dInstrumentalness*dInstrumentalness +
			dValence*dValence +
			dTempo*dTempo,
	)

	return distance
}

// normalizeAudioFeatures normalizes audio features to a common scale.
func normalizeAudioFeatures(af *models.AudioFeatures) models.AudioFeatures {
	normalized := *af

	// Tempo normalization (assuming typical range 40-200 BPM)
	minTempo := 40.0
	maxTempo := 200.0
	normalized.Tempo = (af.Tempo - minTempo) / (maxTempo - minTempo)
	if normalized.Tempo < 0 {
		normalized.Tempo = 0
	}
	if normalized.Tempo > 1 {
		normalized.Tempo = 1
	}

	// Other features are already 0-1, but we can add more normalization if needed
	return normalized
}

// Helper function to format mood name for display
func formatMoodName(mood string) string {
	return fmt.Sprintf("%s%s", strings.ToUpper(mood[:1]), mood[1:])
}

package services

import (
	"strings"
)

// WordSimilarity represents detailed similarity metrics between words
type WordSimilarity struct {
	Distance     int
	ExactMatch   bool
	Ratio        float32
	CommonPrefix int
	CommonSuffix int
}

// calculateWordSimilarity computes detailed similarity metrics between two strings
func calculateWordSimilarity(s1, s2 string) WordSimilarity {
	// Early exact match check
	if s1 == s2 {
		return WordSimilarity{
			Distance:     0,
			ExactMatch:   true,
			Ratio:        1.0,
			CommonPrefix: len(s1),
			CommonSuffix: len(s1),
		}
	}

	// Case-insensitive comparison
	s1Lower := strings.ToLower(s1)
	s2Lower := strings.ToLower(s2)

	if s1Lower == s2Lower {
		return WordSimilarity{
			Distance:     0,
			ExactMatch:   true,
			Ratio:        0.95, // Slightly lower score for case-insensitive match
			CommonPrefix: len(s1),
			CommonSuffix: len(s1),
		}
	}

	// Calculate common prefix and suffix
	prefix := commonPrefixLength(s1Lower, s2Lower)
	suffix := commonSuffixLength(s1Lower[prefix:], s2Lower[prefix:])

	// If strings are very different in length, early exit
	lenDiff := abs(len(s1) - len(s2))
	if lenDiff > 5 { // Threshold for length difference
		return WordSimilarity{
			Distance:     lenDiff,
			ExactMatch:   false,
			Ratio:        0,
			CommonPrefix: prefix,
			CommonSuffix: suffix,
		}
	}

	// Optimize Levenshtein calculation for similar length strings
	distance := optimizedLevenshteinDistance(s1Lower, s2Lower, prefix, suffix)
	maxLen := float32(max(len(s1), len(s2)))
	ratio := 1.0 - (float32(distance) / maxLen)

	return WordSimilarity{
		Distance:     distance,
		ExactMatch:   false,
		Ratio:        ratio,
		CommonPrefix: prefix,
		CommonSuffix: suffix,
	}
}

// optimizedLevenshteinDistance computes Levenshtein distance with optimizations
func optimizedLevenshteinDistance(s1, s2 string, prefix, suffix int) int {
	// Skip common prefix and suffix
	s1 = s1[prefix : len(s1)-suffix]
	s2 = s2[prefix : len(s2)-suffix]

	// Early exits
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Use shorter string as s1 to minimize memory
	if len(s1) > len(s2) {
		s1, s2 = s2, s1
	}

	// Preallocate rows with capacity
	current := make([]int, len(s2)+1)
	previous := make([]int, len(s2)+1)

	// Initialize first row
	for i := range previous {
		previous[i] = i
	}

	// Main computation loop with optimizations
	for i := 1; i <= len(s1); i++ {
		current[0] = i
		for j := 1; j <= len(s2); j++ {
			if s1[i-1] == s2[j-1] {
				current[j] = previous[j-1]
			} else {
				// Optimized min calculation
				minVal := previous[j]
				if current[j-1] < minVal {
					minVal = current[j-1]
				}
				if previous[j-1] < minVal {
					minVal = previous[j-1]
				}
				current[j] = 1 + minVal
			}
		}
		// Swap slices
		previous, current = current, previous
	}

	return previous[len(s2)]
}

// Helper functions
func commonPrefixLength(s1, s2 string) int {
	maxLen := min(len(s1), len(s2))
	for i := 0; i < maxLen; i++ {
		if s1[i] != s2[i] {
			return i
		}
	}
	return maxLen
}

func commonSuffixLength(s1, s2 string) int {
	maxLen := min(len(s1), len(s2))
	for i := 0; i < maxLen; i++ {
		if s1[len(s1)-1-i] != s2[len(s2)-1-i] {
			return i
		}
	}
	return maxLen
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// calculateNameSimilarity computes similarity score between two names
func calculateNameSimilarity(s1, s2 string) float32 {
	similarity := calculateWordSimilarity(s1, s2)

	// If exact match or case-insensitive match
	if similarity.ExactMatch {
		return similarity.Ratio
	}

	// If strings have significant common parts
	if similarity.CommonPrefix > 3 || similarity.CommonSuffix > 3 {
		return similarity.Ratio * 1.1 // Boost score for significant common parts
	}

	// Regular similarity score
	return similarity.Ratio
}

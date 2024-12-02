// services/spaces.go
package services

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
)

type PathType string

const (
	PathTypeCards PathType = "cards"
	PathTypePromo PathType = "promo"
)

type PathInfo struct {
	BaseDir  PathType
	GroupDir string
	ColID    string
}

type PathCache struct {
	paths map[string]PathInfo
	mu    sync.RWMutex
}

type SpacesService struct {
	client    *s3.Client
	bucket    string
	region    string
	CardRoot  string
	pathCache *PathCache
}

func NewSpacesService(spacesKey, spacesSecret, region, bucket, cardRoot string) *SpacesService {
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: fmt.Sprintf("https://%s.digitaloceanspaces.com", region),
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(spacesKey, spacesSecret, "")),
		config.WithRegion(region),
	)
	if err != nil {
		panic(fmt.Sprintf("Unable to load Spaces config: %v", err))
	}

	client := s3.NewFromConfig(cfg)

	service := &SpacesService{
		client:   client,
		bucket:   bucket,
		region:   region,
		CardRoot: cardRoot,
		pathCache: &PathCache{
			paths: make(map[string]PathInfo),
		},
	}

	go service.initializePathCache()
	return service
}

func (s *SpacesService) initializePathCache() {
	fmt.Println("🔍 Initializing path cache...")
	ctx := context.Background()

	// Define the directory structure
	directories := []struct {
		baseDir   PathType
		groupType string
		fullPath  string
	}{
		{PathTypeCards, "girlgroups", "girlgroups"},
		{PathTypeCards, "boygroups", "boygroups"},
		{PathTypePromo, "girlgroups", "promo/girlgroups"},
	}

	// Use buffered channel for concurrent processing
	resultChan := make(chan PathInfo, 100)
	var wg sync.WaitGroup

	// Process directories concurrently
	for _, dir := range directories {
		wg.Add(1)
		go func(dir struct {
			baseDir   PathType
			groupType string
			fullPath  string
		}) {
			defer wg.Done()
			prefix := fmt.Sprintf("%s/%s/", s.CardRoot, dir.fullPath)

			input := &s3.ListObjectsV2Input{
				Bucket:  &s.bucket,
				Prefix:  &prefix,
				MaxKeys: aws.Int32(1000), // Optimize batch size
			}

			paginator := s3.NewListObjectsV2Paginator(s.client, input)
			for paginator.HasMorePages() {
				if output, err := paginator.NextPage(ctx); err == nil {
					for _, obj := range output.Contents {
						if obj.Key == nil {
							continue
						}

						path := strings.TrimPrefix(*obj.Key, s.CardRoot+"/")
						parts := strings.Split(path, "/")

						colIDIndex := 1
						if strings.HasPrefix(path, "promo/") {
							colIDIndex = 2
						}

						if len(parts) > colIDIndex {
							resultChan <- PathInfo{
								BaseDir:  dir.baseDir,
								GroupDir: dir.groupType,
								ColID:    parts[colIDIndex],
							}
						}
					}
				}
			}
		}(dir)
	}

	// Close result channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results
	for pathInfo := range resultChan {
		s.pathCache.mu.Lock()
		s.pathCache.paths[pathInfo.ColID] = pathInfo
		s.pathCache.mu.Unlock()
	}

	fmt.Printf("🏁 Path cache initialization complete. Found %d collections\n", len(s.pathCache.paths))
}

func (s *SpacesService) GetCardImageURL(cardName string, colID string, level int, groupType string) string {
	var baseDir string

	s.pathCache.mu.RLock()
	pathInfo, exists := s.pathCache.paths[colID]
	s.pathCache.mu.RUnlock()

	if exists {
		baseDir = string(pathInfo.BaseDir)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		paths := []string{
			fmt.Sprintf("%s/%s/%s/%d_%s.jpg", s.CardRoot, groupType, colID, level, cardName),
			fmt.Sprintf("%s/promo/%s/%s/%d_%s.jpg", s.CardRoot, groupType, colID, level, cardName),
		}

		for _, path := range paths {
			_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
				Bucket: &s.bucket,
				Key:    &path,
			})
			if err == nil {
				baseDir = strings.Split(path, "/")[1]
				s.pathCache.mu.Lock()
				s.pathCache.paths[colID] = PathInfo{
					BaseDir:  PathType(baseDir),
					GroupDir: groupType,
					ColID:    colID,
				}
				s.pathCache.mu.Unlock()
				break
			}
		}

		if baseDir == "" {
			baseDir = "promo"
		}
	}

	var sb strings.Builder
	sb.WriteString("https://cards.hyejoobot.com/")
	sb.WriteString(baseDir)
	sb.WriteByte('/')
	sb.WriteString(groupType)
	sb.WriteByte('/')
	sb.WriteString(colID)
	sb.WriteByte('/')
	sb.WriteString(fmt.Sprintf("%d_%s.jpg", level, cardName))

	return sb.String()
}

func (s *SpacesService) GetBucket() string {
	return s.bucket
}

func (s *SpacesService) GetRegion() string {
	return s.region
}

func (s *SpacesService) GetCardRoot() string {
	return s.CardRoot
}

func (s *SpacesService) DeleteCardImage(ctx context.Context, colID string, cardName string, level int, tags []string) error {
	// Get group type from tags
	var groupType string
	for _, tag := range tags {
		if tag == "girlgroups" || tag == "boygroups" {
			groupType = tag
			break
		}
	}

	if groupType == "" {
		return fmt.Errorf("invalid card type: neither girlgroups nor boygroups tag found")
	}

	// Check cache first for the correct path
	s.pathCache.mu.RLock()
	pathInfo, exists := s.pathCache.paths[colID]
	s.pathCache.mu.RUnlock()

	var paths []string
	if exists {
		// Use cached path
		paths = []string{
			fmt.Sprintf("%s/%s/%s/%s/%d_%s.jpg",
				s.CardRoot,
				pathInfo.BaseDir,
				groupType,
				colID,
				level,
				cardName,
			),
		}
	} else {
		// Try both possible paths
		paths = []string{
			fmt.Sprintf("%s/cards/%s/%s/%d_%s.jpg", s.CardRoot, groupType, colID, level, cardName),
			fmt.Sprintf("%s/promo/%s/%s/%d_%s.jpg", s.CardRoot, groupType, colID, level, cardName),
		}
	}

	var errors []string
	deleted := false

	// Try deleting from all possible paths
	for _, path := range paths {
		_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: &s.bucket,
			Key:    &path,
		})
		if err == nil {
			deleted = true
			// Remove from cache if exists
			s.pathCache.mu.Lock()
			delete(s.pathCache.paths, colID)
			s.pathCache.mu.Unlock()
			break
		} else {
			errors = append(errors, fmt.Sprintf("path (%s): %v", path, err))
		}
	}

	if !deleted {
		return fmt.Errorf("failed to delete image from any path: %s", strings.Join(errors, "; "))
	}

	return nil
}

// ImageOperation represents the type of image operation
type ImageOperation string

const (
	ImageOperationUpload ImageOperation = "upload"
	ImageOperationUpdate ImageOperation = "update"
	ImageOperationSync   ImageOperation = "sync"
	ImageOperationVerify ImageOperation = "verify"
	DeleteOperation      ImageOperation = "delete"
)

// ImageManagementResult represents the result of an image management operation
type ImageManagementResult struct {
	Operation    ImageOperation
	Success      bool
	CardName     string
	CollectionID string
	Level        int
	URL          string
	ErrorMessage string
	Stats        map[string]interface{}
}

// Add this helper function at the top level
func getImageNameFromPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	filename := parts[len(parts)-1]
	// Remove level number and .jpg extension
	if idx := strings.Index(filename, "_"); idx != -1 {
		return strings.TrimSuffix(filename[idx+1:], ".jpg")
	}
	return ""
}

// WordSimilarity represents detailed similarity metrics between words
type WordSimilarity struct {
	Distance     int
	ExactMatch   bool
	Ratio        float32
	CommonPrefix int
	CommonSuffix int
}

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

// Update the calculateNameSimilarity function to use the new word similarity
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

// ManageCardImage handles various image operations for cards
func (s *SpacesService) ManageCardImage(ctx context.Context, operation ImageOperation, cardID int64, imageData []byte, card *models.Card) (*ImageManagementResult, error) {
	result := &ImageManagementResult{
		Operation:    operation,
		CardName:     card.Name,
		CollectionID: card.ColID,
		Level:        card.Level,
	}

	groupType := "girlgroups"
	for _, tag := range card.Tags {
		if tag == "boygroups" {
			groupType = "boygroups"
			break
		}
	}

	// Try all possible path patterns
	possiblePaths := []string{
		// Standard cards path
		fmt.Sprintf("cards/%s/%s/%d_%s.jpg",
			groupType, card.ColID, card.Level, card.Name),
		// Promo path
		fmt.Sprintf("promo/%s/%s/%d_%s.jpg",
			groupType, card.ColID, card.Level, card.Name),
	}

	var foundPath string
	var err error

	// Try each path until we find one that exists
	for _, path := range possiblePaths {
		_, err = s.client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(path),
		})
		if err == nil {
			foundPath = path
			break
		}
	}

	switch operation {
	case ImageOperationUpload, ImageOperationUpdate:
		// For uploads/updates, use the standard cards path
		foundPath = possiblePaths[0]
		input := &s3.PutObjectInput{
			Bucket:       aws.String(s.bucket),
			Key:          aws.String(foundPath),
			Body:         bytes.NewReader(imageData),
			ContentType:  aws.String("image/jpeg"),
			CacheControl: aws.String("public, max-age=31536000"),
			ACL:          types.ObjectCannedACLPublicRead,
		}

		_, err := s.client.PutObject(ctx, input)
		if err != nil {
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("Failed to upload image: %v", err)
			return result, err
		}

	case ImageOperationVerify:
		// First list all objects in the collection directory
		searchPattern := fmt.Sprintf("cards/%s/%s/", groupType, card.ColID)
		listInput := &s3.ListObjectsV2Input{
			Bucket: aws.String(s.bucket),
			Prefix: aws.String(searchPattern),
		}

		output, err := s.client.ListObjectsV2(ctx, listInput)
		if err != nil {
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("Failed to search for images: %v", err)
			return result, err
		}

		// Look for exact match first, then similar names
		var exactMatch string
		var similarMatches []struct {
			path  string
			name  string
			score float32
		}

		targetName := strings.ToLower(strings.TrimSuffix(card.Name, ".jpg"))

		for _, obj := range output.Contents {
			if obj.Key == nil {
				continue
			}

			imageName := getImageNameFromPath(*obj.Key)
			if imageName == "" {
				continue
			}

			// Check if this is a level match
			objBase := path.Base(*obj.Key)
			if !strings.HasPrefix(objBase, fmt.Sprintf("%d_", card.Level)) {
				continue
			}

			// Compare names
			imageNameLower := strings.ToLower(imageName)

			// First try exact match
			if imageNameLower == targetName {
				exactMatch = *obj.Key
				break
			}

			// If not exact match, calculate similarity
			score := calculateNameSimilarity(targetName, imageNameLower)
			if score >= 0.8 { // Increased threshold for more accurate matching
				similarMatches = append(similarMatches, struct {
					path  string
					name  string
					score float32
				}{
					path:  *obj.Key,
					name:  imageName,
					score: score,
				})
			}
		}

		if exactMatch != "" {
			foundPath = exactMatch
			result.Success = true
		} else if len(similarMatches) > 0 {
			// Sort matches by score (highest first)
			sort.Slice(similarMatches, func(i, j int) bool {
				return similarMatches[i].score > similarMatches[j].score
			})

			// Use the best match
			bestMatch := similarMatches[0]
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("Similar card found!\nDatabase name: %s\nFound image: %s\nPath: %s\nMatch confidence: %.1f%%\n\nDid you mean this card?",
				card.Name,
				bestMatch.name,
				bestMatch.path,
				bestMatch.score*100,
			)
			result.URL = fmt.Sprintf("https://%s.%s.digitaloceanspaces.com/%s", s.bucket, s.region, bestMatch.path)
			return result, nil
		} else {
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("No matching images found for card: %s\nSearched in: %s",
				card.Name,
				searchPattern,
			)
			return result, nil
		}
	}

	// Set success result
	result.Success = true
	result.URL = fmt.Sprintf("https://%s.%s.digitaloceanspaces.com/%s", s.bucket, s.region, foundPath)

	return result, nil
}

func (s *SpacesService) DeleteObject(ctx context.Context, path string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	_, err := s.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete object %s: %w", path, err)
	}

	// Wait for deletion to complete
	waiter := s3.NewObjectNotExistsWaiter(s.client)
	err = waiter.Wait(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}, 30*time.Second)

	if err != nil {
		return fmt.Errorf("error waiting for object deletion %s: %w", path, err)
	}

	// Remove from cache if exists
	s.pathCache.mu.Lock()
	for k, v := range s.pathCache.paths {
		if strings.Contains(path, v.ColID) {
			delete(s.pathCache.paths, k)
			break
		}
	}
	s.pathCache.mu.Unlock()

	return nil
}

// Add this method to SpacesService
func (s *SpacesService) GetSpacesConfig() utils.SpacesConfig {
	return utils.SpacesConfig{
		Bucket:   s.bucket,
		Region:   s.region,
		CardRoot: s.CardRoot,
		GetImageURL: func(cardName string, colID string, level int, groupType string) string {
			return s.GetCardImageURL(cardName, colID, level, groupType)
		},
	}
}

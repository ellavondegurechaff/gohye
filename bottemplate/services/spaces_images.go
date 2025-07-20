package services

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
)

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

// SpacesImageManager handles image operations for the Spaces service
type SpacesImageManager struct {
	client       *s3.Client
	bucket       string
	region       string
	cardRoot     string
	cacheManager *SpacesCacheManager
}

// NewSpacesImageManager creates a new image manager
func NewSpacesImageManager(client *s3.Client, bucket, region, cardRoot string, cacheManager *SpacesCacheManager) *SpacesImageManager {
	return &SpacesImageManager{
		client:       client,
		bucket:       bucket,
		region:       region,
		cardRoot:     cardRoot,
		cacheManager: cacheManager,
	}
}

// ManageCardImage handles various image operations for cards
func (m *SpacesImageManager) ManageCardImage(ctx context.Context, operation ImageOperation, cardID int64, imageData []byte, card *models.Card) (*ImageManagementResult, error) {
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
		_, err = m.client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(m.bucket),
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
			Bucket:       aws.String(m.bucket),
			Key:          aws.String(foundPath),
			Body:         bytes.NewReader(imageData),
			ContentType:  aws.String("image/jpeg"),
			CacheControl: aws.String("public, max-age=31536000"),
			ACL:          types.ObjectCannedACLPublicRead,
		}

		_, err := m.client.PutObject(ctx, input)
		if err != nil {
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("Failed to upload image: %v", err)
			return result, err
		}

	case ImageOperationVerify:
		// First list all objects in the collection directory
		searchPattern := fmt.Sprintf("cards/%s/%s/", groupType, card.ColID)
		listInput := &s3.ListObjectsV2Input{
			Bucket: aws.String(m.bucket),
			Prefix: aws.String(searchPattern),
		}

		output, err := m.client.ListObjectsV2(ctx, listInput)
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
			result.URL = fmt.Sprintf("https://%s.%s.digitaloceanspaces.com/%s", m.bucket, m.region, bestMatch.path)
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
	result.URL = fmt.Sprintf("https://%s.%s.digitaloceanspaces.com/%s", m.bucket, m.region, foundPath)

	return result, nil
}

// DeleteCardImage deletes a card image from the storage
func (m *SpacesImageManager) DeleteCardImage(ctx context.Context, colID string, cardName string, level int, tags []string) error {
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
	pathInfo, exists := m.cacheManager.GetPathInfo(colID)

	var paths []string
	if exists {
		// Use cached path
		paths = []string{
			fmt.Sprintf("%s/%s/%s/%s/%d_%s.jpg",
				m.cardRoot,
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
			fmt.Sprintf("%s/cards/%s/%s/%d_%s.jpg", m.cardRoot, groupType, colID, level, cardName),
			fmt.Sprintf("%s/promo/%s/%s/%d_%s.jpg", m.cardRoot, groupType, colID, level, cardName),
		}
	}

	var errors []string
	deleted := false

	// Try deleting from all possible paths
	for _, path := range paths {
		_, err := m.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: &m.bucket,
			Key:    &path,
		})
		if err == nil {
			deleted = true
			// Remove from cache if exists
			m.cacheManager.RemovePathInfo(colID)
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

// DeleteObject deletes an object at the specified path
func (m *SpacesImageManager) DeleteObject(ctx context.Context, path string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(path),
	}

	_, err := m.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete object %s: %w", path, err)
	}

	// Wait for deletion to complete
	waiter := s3.NewObjectNotExistsWaiter(m.client)
	err = waiter.Wait(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(path),
	}, 30*time.Second)

	if err != nil {
		return fmt.Errorf("error waiting for object deletion %s: %w", path, err)
	}

	// Remove from cache if exists
	for k, v := range m.cacheManager.cache.paths {
		if strings.Contains(path, v.ColID) {
			m.cacheManager.RemovePathInfo(k)
			break
		}
	}

	return nil
}

// Helper function to get image name from path
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

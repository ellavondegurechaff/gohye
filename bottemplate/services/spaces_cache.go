package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

// SpacesCacheManager handles path caching for the Spaces service
type SpacesCacheManager struct {
	client   *s3.Client
	bucket   string
	cardRoot string
	cache    *PathCache
}

// NewSpacesCacheManager creates a new cache manager
func NewSpacesCacheManager(client *s3.Client, bucket, cardRoot string) *SpacesCacheManager {
	return &SpacesCacheManager{
		client:   client,
		bucket:   bucket,
		cardRoot: cardRoot,
		cache: &PathCache{
			paths: make(map[string]PathInfo),
		},
	}
}

// InitializePathCache builds the initial cache of all available paths
func (c *SpacesCacheManager) InitializePathCache() {
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
			prefix := fmt.Sprintf("%s/%s/", c.cardRoot, dir.fullPath)

			input := &s3.ListObjectsV2Input{
				Bucket:  &c.bucket,
				Prefix:  &prefix,
				MaxKeys: aws.Int32(1000), // Optimize batch size
			}

			paginator := s3.NewListObjectsV2Paginator(c.client, input)
			for paginator.HasMorePages() {
				if output, err := paginator.NextPage(ctx); err == nil {
					for _, obj := range output.Contents {
						if obj.Key == nil {
							continue
						}

						path := strings.TrimPrefix(*obj.Key, c.cardRoot+"/")
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
		c.cache.mu.Lock()
		c.cache.paths[pathInfo.ColID] = pathInfo
		c.cache.mu.Unlock()
	}

	fmt.Printf("🏁 Path cache initialization complete. Found %d collections\n", len(c.cache.paths))
}

// GetPathInfo retrieves path information for a collection ID
func (c *SpacesCacheManager) GetPathInfo(colID string) (PathInfo, bool) {
	c.cache.mu.RLock()
	defer c.cache.mu.RUnlock()
	pathInfo, exists := c.cache.paths[colID]
	return pathInfo, exists
}

// UpdatePathInfo updates or adds path information to the cache
func (c *SpacesCacheManager) UpdatePathInfo(colID string, pathInfo PathInfo) {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	c.cache.paths[colID] = pathInfo
}

// RemovePathInfo removes path information from the cache
func (c *SpacesCacheManager) RemovePathInfo(colID string) {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	delete(c.cache.paths, colID)
}

// FindPathForCard attempts to find the correct path for a card by checking existence
func (c *SpacesCacheManager) FindPathForCard(ctx context.Context, cardName string, colID string, level int, groupType string) (string, PathType) {
	// Check cache first
	if pathInfo, exists := c.GetPathInfo(colID); exists {
		return string(pathInfo.BaseDir), pathInfo.BaseDir
	}

	// If not in cache, check both possible paths
	paths := []string{
		fmt.Sprintf("%s/%s/%s/%d_%s.jpg", c.cardRoot, groupType, colID, level, cardName),
		fmt.Sprintf("%s/promo/%s/%s/%d_%s.jpg", c.cardRoot, groupType, colID, level, cardName),
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	for _, path := range paths {
		_, err := c.client.HeadObject(timeoutCtx, &s3.HeadObjectInput{
			Bucket: &c.bucket,
			Key:    &path,
		})
		if err == nil {
			baseDir := strings.Split(path, "/")[1]
			pathType := PathType(baseDir)
			
			// Update cache with found path
			c.UpdatePathInfo(colID, PathInfo{
				BaseDir:  pathType,
				GroupDir: groupType,
				ColID:    colID,
			})
			
			return baseDir, pathType
		}
	}

	// Default to promo if nothing found
	return "promo", PathTypePromo
}

// GetCacheSize returns the number of items in the cache
func (c *SpacesCacheManager) GetCacheSize() int {
	c.cache.mu.RLock()
	defer c.cache.mu.RUnlock()
	return len(c.cache.paths)
}
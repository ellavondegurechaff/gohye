// services/spaces.go
package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
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

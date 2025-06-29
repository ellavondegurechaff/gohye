// services/spaces.go
package services

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
)

type SpacesService struct {
	client       *s3.Client
	bucket       string
	region       string
	CardRoot     string
	cacheManager *SpacesCacheManager
	imageManager *SpacesImageManager
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

	// Initialize component managers
	cacheManager := NewSpacesCacheManager(client, bucket, cardRoot)
	imageManager := NewSpacesImageManager(client, bucket, region, cardRoot, cacheManager)

	service := &SpacesService{
		client:       client,
		bucket:       bucket,
		region:       region,
		CardRoot:     cardRoot,
		cacheManager: cacheManager,
		imageManager: imageManager,
	}

	// Initialize path cache in background
	go service.cacheManager.InitializePathCache()
	
	return service
}

func (s *SpacesService) GetCardImageURL(cardName string, colID string, level int, groupType string) string {
	return s.GetCardImageURLWithFormat(cardName, colID, level, groupType, false)
}

func (s *SpacesService) GetCardImageURLWithFormat(cardName string, colID string, level int, groupType string, animated bool) string {
	// Use cache manager to find the correct path
	baseDir, _ := s.cacheManager.FindPathForCard(context.Background(), cardName, colID, level, groupType)

	// Determine file extension based on animation status
	extension := "jpg"
	if animated {
		extension = "gif"
	}

	var sb strings.Builder
	sb.WriteString("https://cards.hyejoobot.com/")
	sb.WriteString(baseDir)
	sb.WriteByte('/')
	sb.WriteString(groupType)
	sb.WriteByte('/')
	sb.WriteString(colID)
	sb.WriteByte('/')
	sb.WriteString(fmt.Sprintf("%d_%s.%s", level, cardName, extension))

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
	return s.imageManager.DeleteCardImage(ctx, colID, cardName, level, tags)
}

// ManageCardImage handles various image operations for cards
func (s *SpacesService) ManageCardImage(ctx context.Context, operation ImageOperation, cardID int64, imageData []byte, card *models.Card) (*ImageManagementResult, error) {
	return s.imageManager.ManageCardImage(ctx, operation, cardID, imageData, card)
}

func (s *SpacesService) DeleteObject(ctx context.Context, path string) error {
	return s.imageManager.DeleteObject(ctx, path)
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

// UploadFile uploads a file to the specified path in Spaces
func (s *SpacesService) UploadFile(ctx context.Context, data []byte, path string, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:       aws.String(s.bucket),
		Key:          aws.String(path),
		Body:         bytes.NewReader(data),
		ContentType:  aws.String(contentType),
		CacheControl: aws.String("public, max-age=31536000"),
		ACL:          types.ObjectCannedACLPublicRead,
	}

	_, err := s.client.PutObject(ctx, input)
	return err
}

// DeleteFile deletes a file from the specified path in Spaces
func (s *SpacesService) DeleteFile(ctx context.Context, path string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	_, err := s.client.DeleteObject(ctx, input)
	return err
}
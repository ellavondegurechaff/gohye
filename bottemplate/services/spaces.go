// services/spaces.go
package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type SpacesService struct {
	client   *s3.Client
	bucket   string
	region   string
	CardRoot string
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

	return &SpacesService{
		client:   client,
		bucket:   bucket,
		region:   region,
		CardRoot: strings.TrimPrefix(cardRoot, "/"),
	}
}

func (s *SpacesService) DeleteCardImage(ctx context.Context, colID string, cardName string, level int, tags []string) error {
	// Determine the group type from tags
	var groupType string
	for _, tag := range tags {
		if tag == "girlgroups" || tag == "boygroups" {
			groupType = tag
			break
		}
	}

	// If no valid group type found, return an error
	if groupType == "" {
		return fmt.Errorf("invalid card type: neither girlgroups nor boygroups tag found")
	}

	// Create paths for both versions of the image
	legacyPath := fmt.Sprintf("%s/%s/%s/%d_%s.jpg", s.CardRoot, groupType, colID, level, cardName)
	newPath := fmt.Sprintf("%s/%s.jpg", colID, cardName)

	var errors []string
	deleted := false

	// Delete legacy path
	_, err1 := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &legacyPath,
	})
	if err1 == nil {
		deleted = true
	} else {
		errors = append(errors, fmt.Sprintf("legacy path (%s): %v", legacyPath, err1))
	}

	// Delete new path
	_, err2 := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &newPath,
	})
	if err2 == nil {
		deleted = true
	} else {
		errors = append(errors, fmt.Sprintf("new path (%s): %v", newPath, err2))
	}

	// Only return error if no deletions succeeded
	if !deleted {
		return fmt.Errorf("failed to delete images: %s", strings.Join(errors, "; "))
	}

	return nil
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

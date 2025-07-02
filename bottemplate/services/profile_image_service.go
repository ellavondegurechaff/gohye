package services

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type ProfileImageService struct {
	logger *slog.Logger
}

type ProfileData struct {
	Username        string
	AvatarLetter    string
	MemberSince     string
	CardCount       int
	DailyStreak     int
	Rank            string
	IsPremium       bool
	BackgroundImage string
}

func NewProfileImageService() *ProfileImageService {
	service := &ProfileImageService{
		logger: slog.With(slog.String("service", "profile_image")),
	}
	
	// Test chromedp availability
	service.testChromedpAvailability()
	
	return service
}

func (s *ProfileImageService) testChromedpAvailability() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	chromedpCtx, cancel := chromedp.NewContext(ctx)
	defer cancel()
	
	err := chromedp.Run(chromedpCtx, chromedp.Navigate("data:text/html,<html><body>test</body></html>"))
	if err != nil {
		s.logger.Error("chromedp not available - image generation will fail",
			slog.String("error", err.Error()))
	} else {
		s.logger.Info("chromedp is available and working")
	}
}

func (s *ProfileImageService) GenerateProfileImage(ctx context.Context, username string, joinDate time.Time, cardCount int, dailyStreak int, rank string, isPremium bool, backgroundImage string) ([]byte, error) {
	start := time.Now()
	s.logger.Info("Starting profile image generation",
		slog.String("username", username),
		slog.Int("card_count", cardCount),
		slog.Int("daily_streak", dailyStreak))

	// Prepare template data
	avatarLetter := string(strings.ToUpper(username)[0])
	if avatarLetter == "" {
		avatarLetter = "?"
	}

	memberSince := fmt.Sprintf("Member since %s", joinDate.Format("Jan 2006"))

	data := ProfileData{
		Username:        username,
		AvatarLetter:    avatarLetter,
		MemberSince:     memberSince,
		CardCount:       cardCount,
		DailyStreak:     dailyStreak,
		Rank:            rank,
		IsPremium:       isPremium,
		BackgroundImage: backgroundImage,
	}

	// Generate HTML content
	s.logger.Info("Generating HTML content")
	htmlContent, err := s.generateHTML(data)
	if err != nil {
		s.logger.Error("Failed to generate HTML", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to generate HTML: %w", err)
	}
	s.logger.Info("HTML content generated successfully", slog.Int("html_length", len(htmlContent)))

	// Create chromedp context with timeout
	s.logger.Info("Creating chromedp context")
	chromedpCtx, cancel := chromedp.NewContext(ctx, chromedp.WithLogf(func(string, ...interface{}) {}))
	defer cancel()

	// Set timeout for the entire operation
	chromedpCtx, cancel = context.WithTimeout(chromedpCtx, 15*time.Second)
	defer cancel()

	var imageBytes []byte

	// Generate image using chromedp
	s.logger.Info("Starting chromedp operations")
	err = chromedp.Run(chromedpCtx,
		chromedp.Navigate("data:text/html,"+htmlContent),
		chromedp.WaitVisible("#profile-container", chromedp.ByID),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Screenshot("#profile-container", &imageBytes, chromedp.ByID),
	)

	if err != nil {
		s.logger.Error("Failed to generate image with chromedp",
			slog.String("error", err.Error()),
			slog.Duration("elapsed", time.Since(start)))
		return nil, fmt.Errorf("failed to generate image: %w", err)
	}

	s.logger.Info("Profile image generated successfully",
		slog.String("username", username),
		slog.Int("image_size", len(imageBytes)),
		slog.Duration("elapsed", time.Since(start)))

	return imageBytes, nil
}

func (s *ProfileImageService) generateHTML(data ProfileData) (string, error) {
	// Get the template file path
	templatePath := filepath.Join("bottemplate", "templates", "profile.html")
	
	// Read the template file
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		s.logger.Error("Failed to read template file", slog.String("error", err.Error()), slog.String("path", templatePath))
		return "", fmt.Errorf("failed to read template file: %w", err)
	}

	// Create template
	tmpl, err := template.New("profile").Parse(string(templateContent))
	if err != nil {
		s.logger.Error("Failed to parse template", slog.String("error", err.Error()))
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		s.logger.Error("Failed to execute template", slog.String("error", err.Error()))
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	// Minimal HTML processing for faster generation
	htmlContent := strings.ReplaceAll(buf.String(), "#", "%23")
	htmlContent = strings.ReplaceAll(htmlContent, "\n", "")

	s.logger.Info("HTML template processed successfully", slog.Int("content_length", len(htmlContent)))
	return htmlContent, nil
}
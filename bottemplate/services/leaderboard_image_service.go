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
	"github.com/disgoorg/bot-template/bottemplate/database/models"
)

type LeaderboardImageService struct {
	logger *slog.Logger
}

type LeaderboardData struct {
	CollectionName string
	CollectionID   string
	Timestamp      string
	Results        []*models.CollectionProgressResult
}

func NewLeaderboardImageService() *LeaderboardImageService {
	service := &LeaderboardImageService{
		logger: slog.With(slog.String("service", "leaderboard_image")),
	}
	
	// Test chromedp availability
	service.testChromedpAvailability()
	
	return service
}

func (s *LeaderboardImageService) testChromedpAvailability() {
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

func (s *LeaderboardImageService) GenerateLeaderboardImage(ctx context.Context, collectionName string, collectionID string, results []*models.CollectionProgressResult) ([]byte, error) {
	start := time.Now()
	s.logger.Info("Starting leaderboard image generation",
		slog.String("collection", collectionName),
		slog.Int("results_count", len(results)))

	// Check if we have any results
	if len(results) == 0 {
		s.logger.Error("No results provided for image generation")
		return nil, fmt.Errorf("no leaderboard results provided")
	}

	// Limit to top 5 results
	if len(results) > 5 {
		results = results[:5]
	}
	
	s.logger.Info("Processing results for image generation",
		slog.Int("final_results_count", len(results)))

	// Prepare template data
	data := LeaderboardData{
		CollectionName: collectionName,
		CollectionID:   collectionID,
		Timestamp:      time.Now().Format("15:04 MST"),
		Results:        results,
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

	// Set timeout for the entire operation - reduced for faster generation
	chromedpCtx, cancel = context.WithTimeout(chromedpCtx, 15*time.Second)
	defer cancel()

	var imageBytes []byte

	// Generate image using chromedp - optimized for speed
	s.logger.Info("Starting chromedp operations")
	err = chromedp.Run(chromedpCtx,
		chromedp.Navigate("data:text/html,"+htmlContent),
		chromedp.WaitVisible("#leaderboard-container", chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond), // Reduced wait time
		chromedp.Screenshot("#leaderboard-container", &imageBytes, chromedp.ByID),
	)

	if err != nil {
		s.logger.Error("Failed to generate image with chromedp",
			slog.String("error", err.Error()),
			slog.Duration("elapsed", time.Since(start)))
		return nil, fmt.Errorf("failed to generate image: %w", err)
	}

	s.logger.Info("Leaderboard image generated successfully",
		slog.String("collection", collectionName),
		slog.Int("image_size", len(imageBytes)),
		slog.Duration("elapsed", time.Since(start)))

	return imageBytes, nil
}

func (s *LeaderboardImageService) generateHTML(data LeaderboardData) (string, error) {
	// Get the template file path
	templatePath := filepath.Join("bottemplate", "templates", "leaderboard.html")
	
	// Read the template file
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		s.logger.Error("Failed to read template file", slog.String("error", err.Error()), slog.String("path", templatePath))
		return "", fmt.Errorf("failed to read template file: %w", err)
	}

	// Create template with functions
	tmpl, err := template.New("leaderboard").Funcs(template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}).Parse(string(templateContent))
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
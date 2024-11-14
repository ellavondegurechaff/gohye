package utils

import (
	"context"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"sync"
	"time"
)

var (
	// Default fallback color
	defaultColor = 0x2b2d31

	// Aggressive HTTP client settings
	httpClient = &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:          200,
			MaxIdleConnsPerHost:   100,
			IdleConnTimeout:       30 * time.Second,
			DisableCompression:    true, // Disable compression for faster processing
			DisableKeepAlives:     false,
			MaxConnsPerHost:       100,
			ResponseHeaderTimeout: 1 * time.Second,
			ExpectContinueTimeout: 500 * time.Millisecond,
		},
	}

	// Thread-safe cache with TTL
	colorCache = &ColorCache{
		data: make(map[string]CacheEntry),
		mu:   sync.RWMutex{},
	}

	// Background worker channel
	imageChan = make(chan string, 100)
)

type CacheEntry struct {
	Color     int
	Timestamp time.Time
}

type ColorCache struct {
	data map[string]CacheEntry
	mu   sync.RWMutex
}

func init() {
	// Start background worker
	go backgroundWorker()

	// Start cache cleanup
	go cacheCleaner()
}

func GetDominantColor(imageURL string) int {
	// Fast cache check
	colorCache.mu.RLock()
	if entry, exists := colorCache.data[imageURL]; exists && time.Since(entry.Timestamp) < 24*time.Hour {
		colorCache.mu.RUnlock()
		return entry.Color
	}
	colorCache.mu.RUnlock()

	// Queue for background processing if not in cache
	select {
	case imageChan <- imageURL:
	default:
	}

	// Return default immediately
	return defaultColor
}

func backgroundWorker() {
	for imageURL := range imageChan {
		color := processImageURL(imageURL)
		if color != defaultColor {
			colorCache.mu.Lock()
			colorCache.data[imageURL] = CacheEntry{
				Color:     color,
				Timestamp: time.Now(),
			}
			colorCache.mu.Unlock()
		}
	}
}

func processImageURL(imageURL string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return defaultColor
	}

	req.Header.Set("Accept", "image/webp,image/jpeg,image/png,image/*")

	resp, err := httpClient.Do(req)
	if err != nil {
		return defaultColor
	}
	defer resp.Body.Close()

	// Quick decode with size limits
	img, _, err := image.DecodeConfig(resp.Body)
	if err != nil {
		return defaultColor
	}

	// Skip large images immediately
	if img.Width > 512 || img.Height > 512 {
		return defaultColor
	}

	// Decode actual image
	fullImg, _, err := image.Decode(resp.Body)
	if err != nil {
		return defaultColor
	}

	return calculateColor(fullImg, img.Width, img.Height)
}

func calculateColor(img image.Image, width, height int) int {
	// Super aggressive sampling - only sample a few pixels
	samplePoints := []struct{ x, y int }{
		{width / 4, height / 4},
		{width / 2, height / 2},
		{3 * width / 4, 3 * height / 4},
	}

	var r, g, b uint32
	for _, point := range samplePoints {
		pr, pg, pb, _ := img.At(point.x, point.y).RGBA()
		r += pr >> 8
		g += pg >> 8
		b += pb >> 8
	}

	r /= 3
	g /= 3
	b /= 3

	return int(r)<<16 | int(g)<<8 | int(b)
}

func cacheCleaner() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		colorCache.mu.Lock()
		now := time.Now()
		for url, entry := range colorCache.data {
			if now.Sub(entry.Timestamp) > 24*time.Hour {
				delete(colorCache.data, url)
			}
		}
		colorCache.mu.Unlock()
	}
}

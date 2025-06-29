package config

import (
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
)

// WebAppConfig contains web-specific configuration
type WebAppConfig struct {
	Config      *bottemplate.Config
	Debug       bool
	Environment string
}

// CardManagementConfig contains settings for card management operations
type CardManagementConfig struct {
	BatchSize         int           `toml:"batch_size"`
	MaxParallel       int           `toml:"max_parallel"`
	ImageMaxSize      string        `toml:"image_max_size"`
	SupportedFormats  []string      `toml:"supported_formats"`
	NamingStrategy    string        `toml:"naming_strategy"`
	EnablePreview     bool          `toml:"enable_preview"`
	UploadTimeout     time.Duration `toml:"upload_timeout"`
	ProcessingTimeout time.Duration `toml:"processing_timeout"`
}

// DefaultCardManagementConfig returns default configuration for card management
func DefaultCardManagementConfig() CardManagementConfig {
	return CardManagementConfig{
		BatchSize:         100,
		MaxParallel:       5,
		ImageMaxSize:      "10MB",
		SupportedFormats:  []string{"jpg", "jpeg", "png", "gif"},
		NamingStrategy:    "collection_slug",
		EnablePreview:     true,
		UploadTimeout:     30 * time.Minute,
		ProcessingTimeout: 60 * time.Minute,
	}
}

// NewWebAppConfig creates a new web app configuration
func NewWebAppConfig(cfg *bottemplate.Config, debug bool) *WebAppConfig {
	environment := "production"
	if debug {
		environment = "development"
	}
	
	return &WebAppConfig{
		Config:      cfg,
		Debug:       debug,
		Environment: environment,
	}
}

// GetDatabaseConfig returns the database configuration
func (w *WebAppConfig) GetDatabaseConfig() bottemplate.DBConfig {
	return w.Config.DB
}

// GetWebConfig returns the web configuration
func (w *WebAppConfig) GetWebConfig() bottemplate.WebConfig {
	return w.Config.Web
}

// SpacesConfig represents spaces configuration
type SpacesConfig struct {
	Key      string
	Secret   string
	Region   string
	Bucket   string
	CardRoot string
}

// GetSpacesConfig returns the spaces configuration
func (w *WebAppConfig) GetSpacesConfig() SpacesConfig {
	return SpacesConfig{
		Key:      w.Config.Spaces.Key,
		Secret:   w.Config.Spaces.Secret,
		Region:   w.Config.Spaces.Region,
		Bucket:   w.Config.Spaces.Bucket,
		CardRoot: w.Config.Spaces.CardRoot,
	}
}

// GetLogConfig returns the log configuration
func (w *WebAppConfig) GetLogConfig() bottemplate.LogConfig {
	return w.Config.Log
}
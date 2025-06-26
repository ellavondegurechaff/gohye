package utils

import "github.com/disgoorg/bot-template/bottemplate/config"

// Re-export commonly used constants for backward compatibility
const (
	// Pagination
	CardsPerPage = config.CardsPerPage

	// Search Related
	SearchTimeout   = config.SearchTimeout
	CacheExpiration = config.CacheExpiration

	// Market Related
	MarketQueryTimeout = config.MarketQueryTimeout

	// Colors (deprecated - use config.ErrorColor instead)
	ErrorColor   = config.ErrorColor
	SuccessColor = config.SuccessColor

	// UI Elements
	ActiveMarketStatus   = config.ActiveMarketStatus
	InactiveMarketStatus = config.InactiveMarketStatus
)

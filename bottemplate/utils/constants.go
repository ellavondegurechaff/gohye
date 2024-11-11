package utils

import "time"

const (
	// Pagination
	CardsPerPage = 10

	// Search Related
	SearchTimeout   = 10 * time.Second
	CacheExpiration = 5 * time.Minute

	// Market Related
	MarketQueryTimeout = 30 * time.Second
)

// Colors
const (
	ErrorColor   = 0xFF0000
	SuccessColor = 0x00FF00
)

// UI Elements
const (
	ActiveMarketStatus   = "🟢 Active"
	InactiveMarketStatus = "🔴 Inactive"
)

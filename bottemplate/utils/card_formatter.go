package utils

import (
	"fmt"
	"strings"
	"unicode"
)

// CardDisplayInfo represents the formatted display information for a card
type CardDisplayInfo struct {
	FormattedName       string
	FormattedCollection string
	ImageURL            string
	Stars               string
	Hyperlink           string
}

// FormatCardName converts names like "hoot_taeyeon" to "Hoot Taeyeon"
func FormatCardName(name string) string {
	// Early return for empty names
	if name == "" {
		return ""
	}

	parts := strings.Split(name, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		// Capitalize first letter of each part
		r := []rune(part)
		r[0] = unicode.ToUpper(r[0])
		parts[i] = string(r)
	}
	return strings.Join(parts, " ")
}

// FormatCollectionName optimizes collection name formatting
func FormatCollectionName(colID string) string {
	switch strings.ToLower(colID) {
	case "gidle":
		return "[G)I-DLE]"
	case "ioi":
		return "[I.O.I]"
	default:
		return "[" + strings.ToUpper(colID) + "]"
	}
}

// GetCardDisplayInfo formats all card information for display
func GetCardDisplayInfo(cardName string, colID string, level int, groupType string, spacesConfig SpacesConfig) CardDisplayInfo {
	imageURL := spacesConfig.GetImageURL(cardName, colID, level, groupType)
	return CardDisplayInfo{
		FormattedName:       FormatCardName(cardName),
		FormattedCollection: FormatCollectionName(colID),
		ImageURL:            imageURL,
		Stars:               GetStarsDisplay(level),
		Hyperlink:           fmt.Sprintf("[%s](%s)", FormatCardName(cardName), imageURL),
	}
}

// GetStarsDisplay returns stars based on level (1-5)
func GetStarsDisplay(level int) string {
	if level < 1 || level > 5 {
		return "`✧`"
	}
	return fmt.Sprintf("`%s`", strings.Repeat("★", level))
}

// formatTags optimizes group type formatting
func formatTags(groupType string) string {
	switch groupType {
	case "girlgroups":
		return "👯‍♀️ Girl Group"
	case "boygroups":
		return "👯‍♂️ Boy Group"
	default:
		return "👤 Solo Artist"
	}
}

// SpacesConfig holds the configuration for DigitalOcean Spaces
type SpacesConfig struct {
	Bucket      string
	Region      string
	CardRoot    string
	GetImageURL func(cardName string, colID string, level int, groupType string) string
}

// FormatCardEntry formats a single card entry with hyperlink and icons
func FormatCardEntry(displayInfo CardDisplayInfo, favorite bool, animated bool, amount int, extraInfo ...string) string {
	var icons strings.Builder

	if favorite {
		icons.WriteString(" `❤️`")
	}
	if animated {
		icons.WriteString(" `✨`")
	}
	if amount > 1 {
		icons.WriteString(fmt.Sprintf(" `x%d`", amount))
	}

	// Add any extra info (like diff percentage, miss count, etc.)
	for _, info := range extraInfo {
		if info != "" {
			icons.WriteString(" " + info)
		}
	}

	return fmt.Sprintf("* %s %s%s `[%s]`",
		displayInfo.Stars,
		displayInfo.Hyperlink,
		icons.String(),
		strings.Trim(displayInfo.FormattedCollection, "[]"),
	)
}

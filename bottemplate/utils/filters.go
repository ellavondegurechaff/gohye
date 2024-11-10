package utils

import (
	"fmt"
	"strings"

	"github.com/disgoorg/disgo/discord"
)

// Common filter options that can be reused across commands
var CommonFilterOptions = []discord.ApplicationCommandOption{
	discord.ApplicationCommandOptionString{
		Name:        "name",
		Description: "Filter by card name",
		Required:    false,
	},
	discord.ApplicationCommandOptionInt{
		Name:        "level",
		Description: "Filter by card level (1-5)",
		Required:    false,
		Choices: []discord.ApplicationCommandOptionChoiceInt{
			{Name: "5 ⭐", Value: 5},
			{Name: "4 ⭐", Value: 4},
			{Name: "3 ⭐", Value: 3},
			{Name: "2 ⭐", Value: 2},
			{Name: "1 ⭐", Value: 1},
		},
	},
	discord.ApplicationCommandOptionString{
		Name:        "tags",
		Description: "Filter by card type",
		Required:    false,
		Choices: []discord.ApplicationCommandOptionChoiceString{
			{Name: "Girl Groups", Value: "girlgroups"},
			{Name: "Boy Groups", Value: "boygroups"},
		},
	},
	discord.ApplicationCommandOptionString{
		Name:        "collection",
		Description: "Filter by collection ID",
		Required:    false,
	},
	discord.ApplicationCommandOptionBool{
		Name:        "animated",
		Description: "Filter animated cards only",
		Required:    false,
	},
}

// FilterInfo holds all possible filter criteria
type FilterInfo struct {
	Name       string
	Level      int
	Tags       string
	Collection string
	Animated   bool
	Favorites  bool // Only used in cards command
}

// BuildFilterDescription creates a formatted string of active filters
func BuildFilterDescription(filters FilterInfo) string {
	if !HasActiveFilters(filters) {
		return ""
	}

	var filterLines []string

	if filters.Name != "" {
		filterLines = append(filterLines, formatFilterLine("📝 Name", filters.Name))
	}
	if filters.Level != 0 {
		filterLines = append(filterLines, formatFilterLine("⭐ Level", filters.Level))
	}
	if filters.Tags != "" {
		filterLines = append(filterLines, formatFilterLine("🏷️ Tags", filters.Tags))
	}
	if filters.Collection != "" {
		filterLines = append(filterLines, formatFilterLine("📑 Collection", filters.Collection))
	}
	if filters.Animated {
		filterLines = append(filterLines, "✨ Animated Only")
	}
	if filters.Favorites {
		filterLines = append(filterLines, "❤️ Favorites Only")
	}

	return "```md\n# Active Filters\n* " + strings.Join(filterLines, "\n* ") + "\n```"
}

// HasActiveFilters checks if any filters are active
func HasActiveFilters(filters FilterInfo) bool {
	return filters.Name != "" ||
		filters.Level != 0 ||
		filters.Tags != "" ||
		filters.Collection != "" ||
		filters.Animated ||
		filters.Favorites
}

// FormatCardType converts internal type names to display names
func FormatCardType(cardType string) string {
	switch cardType {
	case "girlgroups":
		return "👯‍♀️ Girl Groups"
	case "boygroups":
		return "👯‍♂️ Boy Groups"
	case "soloist":
		return "👤 Solo Artist"
	default:
		return cardType
	}
}

func formatFilterLine(label string, value interface{}) string {
	return fmt.Sprintf("%s: %v", label, value)
}

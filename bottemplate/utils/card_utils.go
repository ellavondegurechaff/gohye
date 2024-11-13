package utils

// GetRarityName returns the rarity name for a given level
func GetRarityName(level int) string {
	rarities := map[int]string{
		1: "Common",
		2: "Uncommon",
		3: "Rare",
		4: "Epic",
		5: "Legendary",
	}
	if name, ok := rarities[level]; ok {
		return name
	}
	return "Unknown"
}

// GetColorByLevel returns the color code for a given level
func GetColorByLevel(level int) int {
	colors := map[int]int{
		1: 0x808080, // Gray
		2: 0x00FF00, // Green
		3: 0x0000FF, // Blue
		4: 0x800080, // Purple
		5: 0xFFD700, // Gold
	}
	if color, ok := colors[level]; ok {
		return color
	}
	return 0x808080
}

// GetAnimatedTag returns the animated tag if the card is animated
func GetAnimatedTag(animated bool) string {
	if animated {
		return "✨ Animated"
	}
	return ""
}

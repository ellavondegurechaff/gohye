package utils

import (
	"fmt"
	"html/template"
	"strings"
	"time"
)

// TemplateFuncs returns a map of functions that can be used in templates
func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatTime":     formatTime,
		"formatDuration": formatDuration,
		"formatBytes":    formatBytes,
		"capitalize":     capitalize,
		"truncate":       truncate,
		"pluralize":      pluralize,
		"safeHTML":       safeHTML,
		"joinTags":       joinTags,
		"percentage":     percentage,
		"statusColor":    statusColor,
		"levelBadge":     levelBadge,
		"add":            add,
		"subtract":       subtract,
		"multiply":       multiply,
		"divide":         divide,
		"formatNumber":   formatNumber,
		"contains":       contains,
		"hasPrefix":      hasPrefix,
		"hasSuffix":      hasSuffix,
	}
}

// formatTime formats a time according to the given layout
func formatTime(t time.Time, layout string) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(layout)
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	return fmt.Sprintf("%.1fd", d.Hours()/24)
}

// formatBytes formats bytes in a human-readable way
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// capitalize capitalizes the first letter of a string
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

// truncate truncates a string to the specified length
func truncate(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}

// pluralize returns the singular or plural form of a word based on count
func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

// safeHTML returns HTML content that won't be escaped
func safeHTML(s string) template.HTML {
	return template.HTML(s)
}

// joinTags joins a slice of tags with commas
func joinTags(tags []string) string {
	return strings.Join(tags, ", ")
}

// percentage calculates percentage
func percentage(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return (float64(part) / float64(total)) * 100
}

// statusColor returns a CSS class based on status
func statusColor(status string) string {
	switch strings.ToLower(status) {
	case "synced", "healthy", "completed", "success":
		return "text-green-600"
	case "missing_files", "extra_files", "warning":
		return "text-yellow-600"
	case "inconsistent", "failed", "error":
		return "text-red-600"
	case "processing", "uploading":
		return "text-blue-600"
	default:
		return "text-gray-600"
	}
}

// levelBadge returns a CSS class for card level badges
func levelBadge(level int) string {
	switch level {
	case 1:
		return "bg-gray-100 text-gray-800"
	case 2:
		return "bg-green-100 text-green-800"
	case 3:
		return "bg-blue-100 text-blue-800"
	case 4:
		return "bg-purple-100 text-purple-800"
	case 5:
		return "bg-yellow-100 text-yellow-800"
	default:
		return "bg-gray-100 text-gray-800"
	}
}

// add adds two numbers
func add(a, b int) int {
	return a + b
}

// subtract subtracts two numbers
func subtract(a, b int) int {
	return a - b
}

// multiply multiplies two numbers
func multiply(a, b int) int {
	return a * b
}

// divide divides two numbers
func divide(a, b int) int {
	if b == 0 {
		return 0
	}
	return a / b
}

// formatNumber formats a number with thousand separators
func formatNumber(n int64) string {
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}
	
	var result strings.Builder
	for i, char := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result.WriteString(",")
		}
		result.WriteRune(char)
	}
	
	return result.String()
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// hasPrefix checks if a string has a prefix
func hasPrefix(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

// hasSuffix checks if a string has a suffix
func hasSuffix(s, suffix string) bool {
	return strings.HasSuffix(s, suffix)
}
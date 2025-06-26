package admin

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var AnalyzeUsers = discord.SlashCommandCreate{
	Name:        "analyzeusers",
	Description: "📊 Analyze MongoDB users data for migration",
}

// MongoUser represents a user document from MongoDB
type MongoUser struct {
	DiscordID string      `json:"discord_id"`
	Username  string      `json:"username"`
	LastDaily interface{} `json:"lastdaily"`
}

func AnalyzeUsersHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		filePath := "hyejoo2.users.json"

		data, err := os.ReadFile(filePath)
		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Failed to read file: %v", err),
			})
		}

		var users []MongoUser
		if err := json.Unmarshal(data, &users); err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Failed to parse JSON: %v", err),
			})
		}

		analysis := analyzeUsers(users)

		messages := splitAnalysis(analysis)
		for i, msg := range messages {
			embed := discord.NewEmbedBuilder().
				SetTitle(fmt.Sprintf("📊 User Data Analysis (Part %d/%d)", i+1, len(messages))).
				SetDescription(msg).
				SetColor(config.SuccessColor).
				SetTimestamp(time.Now())

			if err := e.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{embed.Build()},
			}); err != nil {
				return err
			}
		}

		return nil
	}
}

func parseMongoDate(v interface{}) (time.Time, error) {
	var t time.Time
	switch v := v.(type) {
	case map[string]interface{}:
		if dateStr, ok := v["$date"].(string); ok {
			if ts, err := time.Parse(time.RFC3339Nano, dateStr); err == nil {
				t = ts
			} else if ts, err := time.Parse("2006-01-02T15:04:05Z07:00", dateStr); err == nil {
				t = ts
			} else {
				return time.Time{}, fmt.Errorf("invalid date string: %v", dateStr)
			}
		} else if dateObj, ok := v["$date"].(map[string]interface{}); ok {
			if msStr, ok := dateObj["$numberLong"].(string); ok {
				msInt, err := strconv.ParseInt(msStr, 10, 64)
				if err != nil {
					return time.Time{}, fmt.Errorf("invalid timestamp: %v", msStr)
				}
				t = time.Unix(0, msInt*int64(time.Millisecond))
			}
		}
	case float64:
		t = time.Unix(0, int64(v)*int64(time.Millisecond))
	case string:
		if ts, err := time.Parse(time.RFC3339Nano, v); err == nil {
			t = ts
		} else if ts, err := time.Parse("2006-01-02T15:04:05Z07:00", v); err == nil {
			t = ts
		}
	}
	return t, nil
}

func analyzeUsers(users []MongoUser) string {
	var analysis strings.Builder

	march2023Start := time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)
	march2023End := time.Date(2023, 3, 31, 23, 59, 59, 0, time.UTC)

	activityByMonth := make(map[string]int)
	var totalMarchUsers int
	var inactiveSinceMarch2023 int
	var active30Days int
	var active60Days int
	var totalUsers int
	var usersToRetain int

	now := time.Now()

	for _, user := range users {
		if user.LastDaily != nil {
			totalUsers++
			lastDailyTime, err := parseMongoDate(user.LastDaily)
			if err == nil {
				monthKey := lastDailyTime.Format("2006-01")
				activityByMonth[monthKey]++

				// Check March 2023 activity
				if lastDailyTime.After(march2023Start) && lastDailyTime.Before(march2023End) {
					totalMarchUsers++
				}

				// Check if inactive since March 2023
				if lastDailyTime.Before(march2023End) {
					inactiveSinceMarch2023++
				} else {
					// Count users to retain (active after March 2023)
					usersToRetain++
				}

				// Check activity windows
				daysSinceActive := now.Sub(lastDailyTime).Hours() / 24
				if daysSinceActive <= 30 {
					active30Days++
				}
				if daysSinceActive <= 60 {
					active60Days++
				}
			}
		}
	}

	var months []string
	for month := range activityByMonth {
		months = append(months, month)
	}
	sort.Strings(months)

	// Build Analysis Output
	analysis.WriteString("## Activity Distribution by Month\n")
	for _, month := range months {
		count := activityByMonth[month]
		analysis.WriteString(fmt.Sprintf("- %s: %d users\n", month, count))
	}

	analysis.WriteString("\n## March 2023 Activity Analysis\n")
	analysis.WriteString(fmt.Sprintf("Total Users in Database: %d\n", totalUsers))
	analysis.WriteString(fmt.Sprintf("Total Users Active in March 2023: %d\n", totalMarchUsers))
	analysis.WriteString(fmt.Sprintf("Users Inactive Since March 2023: %d (%.2f%% of total users)\n",
		inactiveSinceMarch2023,
		float64(inactiveSinceMarch2023)/float64(totalUsers)*100))

	analysis.WriteString("\n## Current Activity Metrics\n")
	analysis.WriteString(fmt.Sprintf("Active Users (Last 30 Days): %d (%.2f%%)\n",
		active30Days,
		float64(active30Days)/float64(totalUsers)*100))
	analysis.WriteString(fmt.Sprintf("Active Users (Last 60 Days): %d (%.2f%%)\n",
		active60Days,
		float64(active60Days)/float64(totalUsers)*100))

	analysis.WriteString("\n## Purge Impact Analysis\n")
	analysis.WriteString(fmt.Sprintf("Users to be Purged (Inactive Since March 2023): %d\n", inactiveSinceMarch2023))
	analysis.WriteString(fmt.Sprintf("Users to Retain (Active After March 2023): %d\n", usersToRetain))
	analysis.WriteString(fmt.Sprintf("Database Size Reduction: %.2f%%\n",
		float64(inactiveSinceMarch2023)/float64(totalUsers)*100))

	return analysis.String()
}

func splitAnalysis(analysis string) []string {
	const maxLength = 4000
	var parts []string

	for len(analysis) > 0 {
		if len(analysis) <= maxLength {
			parts = append(parts, analysis)
			break
		}

		splitIndex := strings.LastIndex(analysis[:maxLength], "\n")
		if splitIndex == -1 {
			splitIndex = maxLength
		}

		parts = append(parts, analysis[:splitIndex])
		analysis = analysis[splitIndex:]
	}

	return parts
}

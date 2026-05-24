package system

import (
	"fmt"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Help = discord.SlashCommandCreate{
	Name:        "help",
	Description: "📖 Display all available commands and their descriptions",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "category",
			Description: "Filter commands by category",
			Required:    false,
			Choices: []discord.ApplicationCommandOptionChoiceString{
				{Name: "Admin", Value: "admin"},
				{Name: "Cards", Value: "cards"},
				{Name: "Economy", Value: "economy"},
				{Name: "Social", Value: "social"},
				{Name: "System", Value: "system"},
			},
		},
	},
}

type HelpHandler struct {
	bot *bottemplate.Bot
}

func NewHelpHandler(b *bottemplate.Bot) *HelpHandler {
	return &HelpHandler{
		bot: b,
	}
}

type CommandInfo struct {
	Name        string
	Description string
	Subcommands []string
}

type CategoryInfo struct {
	Name        string
	Description string
	Commands    []CommandInfo
	Color       int
	Emoji       string
}

func (h *HelpHandler) Handle(event *handler.CommandEvent) error {
	categoryFilter := ""
	if data, ok := event.SlashCommandInteractionData().OptString("category"); ok {
		categoryFilter = data
	}

	categories := h.getCommandCategories()

	if categoryFilter != "" {
		return h.showCategoryHelp(event, categoryFilter, categories)
	}

	return h.showOverviewHelp(event, categories)
}

func (h *HelpHandler) showOverviewHelp(event *handler.CommandEvent, categories map[string]CategoryInfo) error {
	embed := discord.NewEmbedBuilder().
		SetTitle("📖 GoHYE Bot - Command Help").
		SetDescription("**GoHYE** is a K-pop card trading bot with a complete economic system.\nSelect a category to view detailed command information.").
		SetColor(0x7289DA)

	var totalCommands int
	for _, category := range categories {
		totalCommands += len(category.Commands)

		var commandList []string
		for _, cmd := range category.Commands {
			commandList = append(commandList, fmt.Sprintf("`/%s`", cmd.Name))
		}

		fieldValue := fmt.Sprintf("%s **%d commands**\n%s",
			category.Emoji,
			len(category.Commands),
			strings.Join(commandList, " • "))

		embed.AddField(category.Name, fieldValue, false)
	}

	embed.SetFooter(fmt.Sprintf("Total: %d commands • Use /help category:<name> for details", totalCommands), "")

    ownerID := event.User().ID.String()
    components := []discord.ContainerComponent{
        discord.NewActionRow(
            discord.NewStringSelectMenu("help_category/"+ownerID, "Select a category for detailed help...",
                discord.StringSelectMenuOption{
                    Label:       "🛠️ Admin Commands",
                    Value:       "admin",
					Description: "Database and management commands",
				},
				discord.StringSelectMenuOption{
					Label:       "🎴 Card Commands",
					Value:       "cards",
					Description: "Card collection and management",
				},
				discord.StringSelectMenuOption{
					Label:       "💰 Economy Commands",
					Value:       "economy",
					Description: "Trading, auctions, and finances",
				},
				discord.StringSelectMenuOption{
					Label:       "👥 Social Commands",
					Value:       "social",
					Description: "Compare and interact with others",
				},
				discord.StringSelectMenuOption{
					Label:       "⚙️ System Commands",
					Value:       "system",
					Description: "Bot utilities and information",
				},
			),
		),
	}

	return event.CreateMessage(discord.MessageCreate{
		Embeds:     []discord.Embed{embed.Build()},
		Components: components,
	})
}

func (h *HelpHandler) showCategoryHelp(event *handler.CommandEvent, categoryName string, categories map[string]CategoryInfo) error {
	category, exists := categories[categoryName]
	if !exists {
		return utils.EH.CreateErrorEmbed(event, fmt.Sprintf("Category '%s' not found.", categoryName))
	}

	embed := discord.NewEmbedBuilder().
		SetTitle(fmt.Sprintf("%s %s Commands", category.Emoji, category.Name)).
		SetDescription(category.Description).
		SetColor(category.Color)

	for _, cmd := range category.Commands {
		fieldName := fmt.Sprintf("/%s", cmd.Name)
		fieldValue := cmd.Description

		if len(cmd.Subcommands) > 0 {
			fieldValue += fmt.Sprintf("\n**Subcommands:** %s", strings.Join(cmd.Subcommands, ", "))
		}

		embed.AddField(fieldName, fieldValue, false)
	}

	embed.SetFooter(fmt.Sprintf("%d commands in %s category • Use /help to see all categories", len(category.Commands), category.Name), "")

    ownerID := event.User().ID.String()
    components := []discord.ContainerComponent{
        discord.NewActionRow(
            discord.NewPrimaryButton("← Back to Overview", "help_back/"+ownerID).
                WithEmoji(discord.ComponentEmoji{Name: "📖"}),
        ),
    }

	return event.CreateMessage(discord.MessageCreate{
		Embeds:     []discord.Embed{embed.Build()},
		Components: components,
	})
}

func (h *HelpHandler) HandleComponent(event *handler.ComponentEvent) error {
    customID := event.Data.CustomID()

    switch {
    case strings.HasPrefix(customID, "help_category/"):
        parts := strings.Split(customID, "/")
        if len(parts) < 2 || parts[1] != "help_category" {
            return nil
        }
        if len(parts) < 3 || parts[2] != event.User().ID.String() {
            return utils.EH.CreateEphemeralError(event, "Only the command user can use these buttons.")
        }
        data, ok := event.Data.(discord.StringSelectMenuInteractionData)
        if ok && len(data.Values) > 0 {
            categoryName := data.Values[0]
            categories := h.getCommandCategories()
            return h.updateCategoryHelp(event, categoryName, categories)
        }
    case strings.HasPrefix(customID, "help_back/"):
        parts := strings.Split(customID, "/")
        if len(parts) < 3 || parts[2] != event.User().ID.String() {
            return utils.EH.CreateEphemeralError(event, "Only the command user can use these buttons.")
        }
        categories := h.getCommandCategories()
        return h.updateOverviewHelp(event, categories)
    }

    return nil
}

func (h *HelpHandler) updateCategoryHelp(event *handler.ComponentEvent, categoryName string, categories map[string]CategoryInfo) error {
	category, exists := categories[categoryName]
	if !exists {
		return utils.EH.CreateEphemeralError(event, fmt.Sprintf("Category '%s' not found.", categoryName))
	}

	embed := discord.NewEmbedBuilder().
		SetTitle(fmt.Sprintf("%s %s Commands", category.Emoji, category.Name)).
		SetDescription(category.Description).
		SetColor(category.Color)

	for _, cmd := range category.Commands {
		fieldName := fmt.Sprintf("/%s", cmd.Name)
		fieldValue := cmd.Description

		if len(cmd.Subcommands) > 0 {
			fieldValue += fmt.Sprintf("\n**Subcommands:** %s", strings.Join(cmd.Subcommands, ", "))
		}

		embed.AddField(fieldName, fieldValue, false)
	}

	embed.SetFooter(fmt.Sprintf("%d commands in %s category • Use /help to see all categories", len(category.Commands), category.Name), "")

    // Preserve the original owner ID if present in the triggering component
    ownerID := ""
    if idd, ok := event.Data.(interface{ CustomID() string }); ok {
        parts := strings.Split(idd.CustomID(), "/")
        if len(parts) >= 3 {
            ownerID = parts[2]
        }
    }
    components := []discord.ContainerComponent{
        discord.NewActionRow(
            discord.NewPrimaryButton("← Back to Overview", "help_back/"+ownerID).
                WithEmoji(discord.ComponentEmoji{Name: "📖"}),
        ),
    }

	return event.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed.Build()},
		Components: &components,
	})
}

func (h *HelpHandler) updateOverviewHelp(event *handler.ComponentEvent, categories map[string]CategoryInfo) error {
	embed := discord.NewEmbedBuilder().
		SetTitle("📖 GoHYE Bot - Command Help").
		SetDescription("**GoHYE** is a K-pop card trading bot with a complete economic system.\nSelect a category to view detailed command information.").
		SetColor(0x7289DA)

	var totalCommands int
	for _, category := range categories {
		totalCommands += len(category.Commands)

		var commandList []string
		for _, cmd := range category.Commands {
			commandList = append(commandList, fmt.Sprintf("`/%s`", cmd.Name))
		}

		fieldValue := fmt.Sprintf("%s **%d commands**\n%s",
			category.Emoji,
			len(category.Commands),
			strings.Join(commandList, " • "))

		embed.AddField(category.Name, fieldValue, false)
	}

	embed.SetFooter(fmt.Sprintf("Total: %d commands • Use /help category:<name> for details", totalCommands), "")

    ownerID := event.User().ID.String()
    components := []discord.ContainerComponent{
        discord.NewActionRow(
            discord.NewStringSelectMenu("help_category/"+ownerID, "Select a category for detailed help...",
                discord.StringSelectMenuOption{
                    Label:       "🛠️ Admin Commands",
                    Value:       "admin",
					Description: "Database and management commands",
				},
				discord.StringSelectMenuOption{
					Label:       "🎴 Card Commands",
					Value:       "cards",
					Description: "Card collection and management",
				},
				discord.StringSelectMenuOption{
					Label:       "💰 Economy Commands",
					Value:       "economy",
					Description: "Trading, auctions, and finances",
				},
				discord.StringSelectMenuOption{
					Label:       "👥 Social Commands",
					Value:       "social",
					Description: "Compare and interact with others",
				},
				discord.StringSelectMenuOption{
					Label:       "⚙️ System Commands",
					Value:       "system",
					Description: "Bot utilities and information",
				},
			),
		),
	}

	return event.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed.Build()},
		Components: &components,
	})
}

func (h *HelpHandler) getCommandCategories() map[string]CategoryInfo {
	return map[string]CategoryInfo{
		"admin": {
			Name:        "Admin",
			Description: "Administrative commands for bot management and database operations. Most require special permissions.",
			Color:       0xFF6B6B,
			Emoji:       "🛠️",
			Commands: []CommandInfo{
				{Name: "analyze-economy", Description: "📊 Analyze the current economic state"},
				{Name: "analyzeusers", Description: "📊 Analyze MongoDB users data for migration"},
				{Name: "dbtest", Description: "Test database connectivity and operations"},
				{Name: "deletecard", Description: "Permanently delete a card and remove it from all users"},
				{Name: "fixduplicates", Description: "🛠️ Fix duplicate cards in all collections"},
				{Name: "init", Description: "Initialize database tables if they don't exist"},
				{Name: "manage-images", Description: "🖼️ Manage card images", Subcommands: []string{"update", "verify", "delete"}},
			},
		},
		"cards": {
			Name:        "Cards",
			Description: "Core card collection features including claiming, viewing, upgrading, and managing your card inventory.",
			Color:       0x4ECDC4,
			Emoji:       "🎴",
			Commands: []CommandInfo{
				{Name: "cards", Description: "View your card collection"},
				{Name: "claim", Description: "✨ Claim cards from the collection!"},
				{Name: "forge", Description: "✨ Forge two cards into a new one"},
				{Name: "levelup", Description: "Level up or combine your cards"},
				{Name: "limitedcards", Description: "🎴 List all unowned cards from limited collection"},
				{Name: "limitedstats", Description: "📊 View ownership statistics for limited collection cards"},
				{Name: "searchcards", Description: "🔍 Search through the card collection with various filters"},
				{Name: "summon", Description: "✨ Summon a card from your collection"},
			},
		},
		"economy": {
			Name:        "Economy",
			Description: "Financial system commands including trading, auctions, daily rewards, and market statistics.",
			Color:       0xFFD93D,
			Emoji:       "💰",
			Commands: []CommandInfo{
				{Name: "auction", Description: "Auction related commands", Subcommands: []string{"create", "list", "bid", "cancel"}},
				{Name: "balance", Description: "💰 View your current balance and earnings"},
				{Name: "daily", Description: "Claim your daily reward!"},
				{Name: "liquefy", Description: "Convert a card into vials"},
				{Name: "price-stats", Description: "📊 View detailed price statistics for a card"},
				{Name: "shop", Description: "Browse and purchase items from the shop"},
				{Name: "work", Description: "💼 Work in the K-pop industry to earn rewards"},
			},
		},
		"social": {
			Name:        "Social",
			Description: "Interactive features for comparing collections, checking ownership, and managing wishlists with other users.",
			Color:       0xA8E6CF,
			Emoji:       "👥",
			Commands: []CommandInfo{
				{Name: "diff", Description: "Compare card collections between users", Subcommands: []string{"for", "missing"}},
				{Name: "has", Description: "Check if a user has a specific card"},
				{Name: "miss", Description: "View missing cards from your collection"},
				{Name: "wish", Description: "Manage your card wishlist", Subcommands: []string{"list", "add", "remove"}},
			},
		},
		"system": {
			Name:        "System",
			Description: "Bot utilities including inventory management, effects system, performance metrics, and version information.",
			Color:       0xB4A7D6,
			Emoji:       "⚙️",
			Commands: []CommandInfo{
				{Name: "craft-effect", Description: "Craft an effect using recipe cards"},
				{Name: "help", Description: "📖 Display all available commands and their descriptions"},
				{Name: "inventory", Description: "View your inventory of items"},
				{Name: "metrics", Description: "📊 View bot performance metrics and statistics"},
				{Name: "use-effect", Description: "Use an active effect from your inventory"},
				{Name: "version", Description: "Display bot version and commit information"},
			},
		},
	}
}

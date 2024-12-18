package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy/effects"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Shop = discord.SlashCommandCreate{
	Name:        "shop",
	Description: "Browse and purchase items from the shop",
}

type ShopHandler struct {
	bot           *bottemplate.Bot
	effectManager *effects.Manager
}

func NewShopHandler(b *bottemplate.Bot, effectManager *effects.Manager) *ShopHandler {
	return &ShopHandler{
		bot:           b,
		effectManager: effectManager,
	}
}

func (h *ShopHandler) Handle(event *handler.CommandEvent) error {
	return h.handleList(event)
}

func (h *ShopHandler) handleList(event *handler.CommandEvent) error {
	ctx := context.Background()

	items, err := h.effectManager.ListEffectItems(ctx)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Failed to fetch shop items: %v", err),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	actives, _, passives := groupItems(items)
	currentItems := actives
	title := "Shop - Active Effects"

	if len(currentItems) == 0 {
		currentItems = passives
		title = "Shop - Passive Effects"
	}

	components := []discord.ContainerComponent{
		createShopComponents("active")[0],
		createItemSelectMenu(currentItems, "active"),
	}

	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Title:       title,
			Description: "Select an item to view details",
			Color:       getColorByType(currentItems[0].Type),
			Footer: &discord.EmbedFooter{
				Text: "Prices update hourly",
			},
		}},
		Components: components,
	})
}

func createCategoryEmbed(title string, items []*models.EffectItem) discord.Embed {
	fields := make([]discord.EmbedField, 0)
	var color int

	if len(items) == 0 {
		return discord.Embed{
			Title:       title,
			Description: "No items available in this category",
			Color:       0x2b2d31, // Default dark theme color
			Footer: &discord.EmbedFooter{
				Text: "Prices update hourly",
			},
		}
	}

	color = getColorByType(items[0].Type)

	for _, item := range items {
		recipeDisplay := ""
		if len(item.Recipe) > 0 {
			levels := make([]string, 0)
			for _, level := range item.Recipe {
				levels = append(levels, fmt.Sprintf("%d", level))
			}
			recipeDisplay = fmt.Sprintf("\n**Recipe**: [%s]", strings.Join(levels, ", "))
		}

		fields = append(fields, discord.EmbedField{
			Name: fmt.Sprintf("%s **%s**",
				getTypeEmoji(item.Type),
				item.Name,
			),
			Value: fmt.Sprintf("**Price**: %d %s\n**Duration**: %dh%s",
				item.Price,
				getCurrencyEmoji(item.Currency),
				item.Duration,
				recipeDisplay,
			),
			Inline: utils.Ptr(true),
		})
	}

	return discord.Embed{
		Title:       title,
		Description: "Select an item to purchase",
		Fields:      fields,
		Color:       color,
		Footer: &discord.EmbedFooter{
			Text: "Prices update hourly",
		},
	}
}

func getTypeEmoji(itemType string) string {
	switch itemType {
	case models.EffectTypeActive:
		return "⚔️"
	case models.EffectTypePassive:
		return "🛡️"
	case models.EffectTypeRecipe:
		return "📜"
	default:
		return "🎁"
	}
}

func getTypeDescription(itemType string) string {
	switch itemType {
	case models.EffectTypeActive:
		return "[Active Effect Card]"
	case models.EffectTypePassive:
		return "[Passive Effect Card]"
	case models.EffectTypeRecipe:
		return "[Recipe Effect Card]"
	default:
		return "[Consumable Item]"
	}
}

func getColorByType(itemType string) int {
	switch itemType {
	case models.EffectTypeActive:
		return 0xed4245 // Red
	case models.EffectTypePassive:
		return 0x57f287 // Green
	case models.EffectTypeRecipe:
		return 0x5865f2 // Blue
	default:
		return 0xfee75c // Yellow
	}
}

func (h *ShopHandler) handleBuy(event *handler.ComponentEvent) error {
	customID := event.Data.CustomID()
	itemID := strings.TrimPrefix(customID, "shop_buy:")

	ctx := context.Background()
	err := h.effectManager.PurchaseEffect(ctx, event.User().ID.String(), itemID)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("❌ Failed to purchase item: %v", err),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return event.CreateMessage(discord.MessageCreate{
		Content: "✅ Successfully purchased item!",
		Flags:   discord.MessageFlagEphemeral,
	})
}

func groupItems(items []*models.EffectItem) (actives, recipes, passives []*models.EffectItem) {
	for _, item := range items {
		switch item.Type {
		case models.EffectTypeActive:
			actives = append(actives, item)
		case models.EffectTypeRecipe:
			recipes = append(recipes, item)
		case models.EffectTypePassive:
			passives = append(passives, item)
		}
	}
	return
}

func getCurrencyEmoji(currency string) string {
	switch currency {
	case models.CurrencyTomato:
		return "❄"
	case models.CurrencyVials:
		return "💧"
	default:
		return "💰"
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (h *ShopHandler) HandleComponent(event *handler.ComponentEvent) error {
	customID := event.Data.CustomID()

	switch {
	case customID == "shop_category":
		return h.handleCategorySelect(event)
	case strings.HasPrefix(customID, "shop_item"):
		return h.handleItemSelect(event)
	case strings.HasPrefix(customID, "shop_buy:"):
		return h.handleBuy(event)
	default:
		return nil
	}
}

func (h *ShopHandler) handleCategorySelect(event *handler.ComponentEvent) error {
	var selectedValue string

	// Handle both button and select menu interactions
	switch data := event.Data.(type) {
	case discord.StringSelectMenuInteractionData:
		if len(data.Values) == 0 {
			return event.CreateMessage(discord.MessageCreate{
				Content: "Invalid interaction data",
				Flags:   discord.MessageFlagEphemeral,
			})
		}
		selectedValue = data.Values[0]
	case discord.ButtonInteractionData:
		selectedValue = "active" // Default to active when coming from back button
	default:
		return event.CreateMessage(discord.MessageCreate{
			Content: "Invalid interaction data",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	ctx := context.Background()
	items, err := h.effectManager.ListEffectItems(ctx)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Failed to fetch shop items: %v", err),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	actives, _, passives := groupItems(items)
	var currentItems []*models.EffectItem
	var title string

	switch selectedValue {
	case "active":
		currentItems = actives
		title = "Shop - Active Effects"
	case "passive":
		currentItems = passives
		title = "Shop - Passive Effects"
	}

	if len(currentItems) == 0 {
		return event.CreateMessage(discord.MessageCreate{
			Content: "No items available in this category",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	components := []discord.ContainerComponent{
		createShopComponents(selectedValue)[0],
		createItemSelectMenu(currentItems, selectedValue),
	}

	return event.UpdateMessage(discord.MessageUpdate{
		Embeds: &[]discord.Embed{{
			Title:       title,
			Description: "Select an item to view details",
			Color:       getColorByType(currentItems[0].Type),
			Footer: &discord.EmbedFooter{
				Text: "💡 Tip: Prices update hourly",
			},
		}},
		Components: &components,
	})
}

func createShopComponents(selectedValue string) []discord.ContainerComponent {
	return []discord.ContainerComponent{
		discord.NewActionRow(
			discord.NewStringSelectMenu("shop_category", "Select Category",
				discord.StringSelectMenuOption{
					Label:       "Active Effects",
					Value:       "active",
					Description: "View active effect items",
					Emoji:       &discord.ComponentEmoji{Name: "⚔️"},
					Default:     selectedValue == "active",
				},
				discord.StringSelectMenuOption{
					Label:       "Passive Effects",
					Value:       "passive",
					Description: "View passive effect items",
					Emoji:       &discord.ComponentEmoji{Name: "🛡️"},
					Default:     selectedValue == "passive",
				},
			),
		),
	}
}

func createItemSelectMenu(items []*models.EffectItem, itemType string) discord.ContainerComponent {
	options := make([]discord.StringSelectMenuOption, 0, len(items))
	for _, item := range items {
		options = append(options, discord.StringSelectMenuOption{
			Label:       item.Name,
			Value:       "item_" + item.ID,
			Description: fmt.Sprintf("%d %s - %dh duration", item.Price, getCurrencyEmoji(item.Currency), item.Duration),
			Emoji:       &discord.ComponentEmoji{Name: getTypeEmoji(item.Type)},
		})
	}

	return discord.NewActionRow(
		discord.NewStringSelectMenu("shop_item", "Select Item", options...).
			WithMinValues(1).
			WithMaxValues(1),
	)
}

func (h *ShopHandler) handleItemSelect(event *handler.ComponentEvent) error {
	data, ok := event.Data.(discord.StringSelectMenuInteractionData)
	if !ok || len(data.Values) == 0 {
		return event.CreateMessage(discord.MessageCreate{
			Content: "Invalid interaction data",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	ctx := context.Background()
	itemID := strings.TrimPrefix(data.Values[0], "item_")

	item, err := h.effectManager.GetEffectItem(ctx, itemID)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Failed to fetch item details: %v", err),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Enhanced recipe formatting
	recipeText := ""
	if len(item.Recipe) > 0 {
		stars := strings.Repeat("⭐", int(item.Recipe[0]))
		cardWord := "card"
		if len(item.Recipe) > 1 {
			cardWord = "cards"
		}
		recipeText = fmt.Sprintf("* Required Recipe: %d %s %s (collect %d %d-star %s to craft)",
			len(item.Recipe),
			stars,
			cardWord,
			len(item.Recipe),
			item.Recipe[0],
			cardWord)
	}

	embed := discord.NewEmbedBuilder().
		SetTitle(fmt.Sprintf("%s %s", getTypeEmoji(item.Type), item.Name)).
		SetColor(getColorByType(item.Type)).
		SetDescription(fmt.Sprintf("```md\n## Item Details\n* Description: %s\n* Price: %d %s\n* Duration: %d hours\n%s\n```\n\n> 💡 **Crafting Info**: This item requires specific cards to craft. Check your inventory to see if you have the required cards.",
			item.Description,
			item.Price,
			getCurrencyEmoji(item.Currency),
			item.Duration,
			recipeText)).
		SetFooter("💡 Tip: Prices update hourly", "").
		Build()

	// Simplify component structure similar to liquefy
	actionRow := discord.NewActionRow(
		discord.NewSuccessButton("Buy 🛍️", fmt.Sprintf("shop_buy:%s", item.ID)),
		discord.NewSecondaryButton("Back ↩️", "shop_category"),
	)

	return event.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &[]discord.ContainerComponent{actionRow},
	})
}

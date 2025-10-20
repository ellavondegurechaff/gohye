package economy

import (
	"context"
	"fmt"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy/effects"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

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

func getColorByType(itemType string) int {
	switch itemType {
	case models.EffectTypeActive:
		return config.ErrorColor
	case models.EffectTypePassive:
		return config.SuccessColor
	case models.EffectTypeRecipe:
		return config.InfoColor
	default:
		return config.WarningColor
	}
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
	title := "Shop - Items"

	if len(currentItems) == 0 {
		currentItems = passives
		title = "Shop - Effects"
	}

	components := []discord.ContainerComponent{
		createShopComponents("active")[0],
		createItemSelectMenu(currentItems, "active"),
	}

	// Handle empty shop case with appropriate messaging  
	var embed discord.Embed
	if len(currentItems) == 0 {
		embed = discord.Embed{
			Title:       "🛍️ Shop",
			Description: "The shop is currently empty. No items are available for purchase at this time.",
			Color:       config.WarningColor,
			Footer: &discord.EmbedFooter{
				Text: "💡 Come back later or contact an admin if this seems wrong",
			},
		}
	} else {
		embed = discord.Embed{
			Title:       title,
			Description: "Select an item to view details",
			Color:       getColorByType(currentItems[0].Type),
			Footer: &discord.EmbedFooter{
				Text: "Prices update hourly",
			},
		}
	}

	return event.CreateMessage(discord.MessageCreate{
		Embeds:     []discord.Embed{embed},
		Components: components,
	})
}

func (h *ShopHandler) handleBuy(event *handler.ComponentEvent) error {
	customID := event.Data.CustomID()
	itemID := strings.TrimPrefix(customID, "/shop_buy/")

	ctx := context.Background()
	err := h.effectManager.PurchaseEffect(ctx, event.User().ID.String(), itemID)
	if err != nil {
		flags := discord.MessageFlagEphemeral
		return event.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr(fmt.Sprintf("❌ Failed to purchase item: %v", err)),
			Components: &[]discord.ContainerComponent{},
			Flags:      &flags,
		})
	}

	item, err := h.effectManager.GetEffectItem(ctx, itemID)
	if err != nil {
		flags := discord.MessageFlagEphemeral
		return event.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("✅ Item purchased successfully, but failed to get item details"),
			Components: &[]discord.ContainerComponent{},
			Flags:      &flags,
		})
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("✅ Purchase Successful").
		SetColor(config.SuccessColor).
		SetDescription(fmt.Sprintf("```md\n# Item Purchased\n* %s %s\n* Price: %d %s\n* Duration: %d hours```\n> 💡 **Tip**: Use `/inventory` to view your purchased items!",
			getTypeEmoji(item.Type),
			item.Name,
			item.Price,
			getCurrencyEmoji(item.Currency),
			item.Duration)).
		SetFooter("Item added to your inventory", "").
		Build()

	flags := discord.MessageFlagEphemeral
	return event.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &[]discord.ContainerComponent{},
		Flags:      &flags,
	})
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

func (h *ShopHandler) formatItemDescription(item *models.EffectItem) string {
	// Get effect data to check for tier information
	effectData := effects.GetEffectItemByID(item.ID)
	if effectData != nil && effectData.TierData != nil {
		// This is a tiered effect, show tier progression
		tierInfo := "\n\n📊 **Tier Progression:**\n"
		for i, value := range effectData.TierData.Values {
			tier := i + 1
			stars := ""
			for j := 0; j < tier; j++ {
				stars += "⭐"
			}

			tierInfo += fmt.Sprintf("Tier %d %s: %s\n", tier, stars, h.formatEffectValue(item.ID, value))
		}

		return item.Description + tierInfo
	}

	// Regular item without tiers
	return item.Description
}

func (h *ShopHandler) formatEffectValue(effectID string, value int) string {
	switch effectID {
	case "cakeday":
		return fmt.Sprintf("+%d flakes/claim", value)
	case "holygrail":
		return fmt.Sprintf("+%d vials/liquify", value)
	case "wolfofhyejoo":
		return fmt.Sprintf("%d%% cashback", value)
	case "lambofhyejoo":
		return fmt.Sprintf("%d%% bonus", value)
	case "cherrybloss":
		return fmt.Sprintf("%d%% cheaper", value)
	case "rulerjeanne":
		hours := float64(value) / 100.0
		return fmt.Sprintf("%.1fh cooldown", hours)
	case "youthyouthbyyoung":
		return fmt.Sprintf("+%d%% bonus", value)
	case "kisslater":
		return fmt.Sprintf("+%d%% XP", value)
	default:
		return fmt.Sprintf("+%d", value)
	}
}

func (h *ShopHandler) HandleComponent(event *handler.ComponentEvent) error {
	customID := event.Data.CustomID()

	switch {
	case customID == "/shop_category":
		return h.handleCategorySelect(event)
	case customID == "/shop_item_disabled":
		// Handle disabled select menu interaction gracefully
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ This category is currently empty. Please try the other category.",
			Flags:   discord.MessageFlagEphemeral,
		})
	case strings.HasPrefix(customID, "/shop_item"):
		return h.handleItemSelect(event)
	case strings.HasPrefix(customID, "/shop_buy/"):
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
		title = "Shop - Items"
	case "passive":
		currentItems = passives
		title = "Shop - Effects"
	}

	components := []discord.ContainerComponent{
		createShopComponents(selectedValue)[0],
		createItemSelectMenu(currentItems, selectedValue),
	}

	// Handle empty category case with appropriate messaging
	var embed discord.Embed
	if len(currentItems) == 0 {
		embed = discord.Embed{
			Title:       title,
			Description: "This category is currently empty. No items are available for purchase.",
			Color:       config.WarningColor,
			Footer: &discord.EmbedFooter{
				Text: "💡 Try checking the other category or come back later",
			},
		}
	} else {
		embed = discord.Embed{
			Title:       title,
			Description: "Select an item to view details",
			Color:       getColorByType(currentItems[0].Type),
			Footer: &discord.EmbedFooter{
				Text: "💡 Tip: Prices update hourly",
			},
		}
	}

	return event.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &components,
	})
}

func createShopComponents(selectedValue string) []discord.ContainerComponent {
	return []discord.ContainerComponent{
		discord.NewActionRow(
			discord.NewStringSelectMenu("/shop_category", "Select Category",
				discord.StringSelectMenuOption{
					Label:       "Items",
					Value:       "active",
					Description: "View consumable items",
					Emoji:       &discord.ComponentEmoji{Name: "⚔️"},
					Default:     selectedValue == "active",
				},
				discord.StringSelectMenuOption{
					Label:       "Effects",
					Value:       "passive",
					Description: "View permanent effects",
					Emoji:       &discord.ComponentEmoji{Name: "🛡️"},
					Default:     selectedValue == "passive",
				},
			),
		),
	}
}

func createItemSelectMenu(items []*models.EffectItem, _ string) discord.ContainerComponent {
	// Handle empty items array to prevent Discord API error
	if len(items) == 0 {
		// Return a disabled placeholder select menu
		return discord.NewActionRow(
			discord.NewStringSelectMenu("/shop_item_disabled", "No items available",
				discord.StringSelectMenuOption{
					Label:       "No items in this category",
					Value:       "disabled",
					Description: "This category is currently empty",
					Emoji:       &discord.ComponentEmoji{Name: "❌"},
				},
			).WithDisabled(true),
		)
	}

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
		discord.NewStringSelectMenu("/shop_item", "Select Item", options...).
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

	// Enhanced recipe formatting - show actual recipe breakdown
	recipeText := ""
	if len(item.Recipe) > 0 {
		// Count cards by star level
		starCounts := make(map[int64]int)
		for _, stars := range item.Recipe {
			starCounts[stars]++
		}

		// Build recipe text showing actual requirements
		var recipeParts []string
		for stars := int64(1); stars <= 5; stars++ {
			if count := starCounts[stars]; count > 0 {
				starDisplay := strings.Repeat("⭐", int(stars))
				recipeParts = append(recipeParts, fmt.Sprintf("%dx %s", count, starDisplay))
			}
		}

		if len(recipeParts) > 0 {
			recipeText = fmt.Sprintf("* Required Recipe: %s cards", strings.Join(recipeParts, " + "))
		}
	}

	embed := discord.NewEmbedBuilder().
		SetTitle(fmt.Sprintf("%s %s", getTypeEmoji(item.Type), item.Name)).
		SetColor(getColorByType(item.Type)).
		SetDescription(fmt.Sprintf("%s\n\n**Price:** %d %s\n**Duration:** %d hours\n%s",
			h.formatItemDescription(item),
			item.Price,
			getCurrencyEmoji(item.Currency),
			item.Duration,
			recipeText)).
		SetFooter("💡 Tip: Prices update hourly", "").
		Build()

	// Simplify component structure similar to liquefy
	actionRow := discord.NewActionRow(
		discord.NewSuccessButton("Buy 🛍️", fmt.Sprintf("/shop_buy/%s", item.ID)),
		discord.NewSecondaryButton("Back ↩️", "/shop_category"),
	)

	return event.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &[]discord.ContainerComponent{actionRow},
	})
}

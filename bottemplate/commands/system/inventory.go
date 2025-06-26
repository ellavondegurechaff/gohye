package system

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy/effects"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Inventory = discord.SlashCommandCreate{
	Name:        "inventory",
	Description: "View your inventory of items",
}

type InventoryHandler struct {
	bot           *bottemplate.Bot
	effectManager *effects.Manager
}

func NewInventoryHandler(b *bottemplate.Bot, effectManager *effects.Manager) *InventoryHandler {
	return &InventoryHandler{
		bot:           b,
		effectManager: effectManager,
	}
}

func (h *InventoryHandler) Handle(event *handler.CommandEvent) error {
	return h.handleList(event)
}

func (h *InventoryHandler) handleList(event *handler.CommandEvent) error {
	ctx := context.Background()
	userID := event.User().ID.String()

	items, err := h.effectManager.ListUserEffects(ctx, userID)
	if err != nil {
		return utils.EH.CreateErrorEmbed(event, fmt.Sprintf("Failed to fetch inventory: %v", err))
	}

	if len(items) == 0 {
		return event.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{{
				Title:       "📦 Your Inventory",
				Description: "Your inventory is empty! Visit the `/shop` to purchase items.",
				Color:       0x2b2d31,
				Footer: &discord.EmbedFooter{
					Text: "💡 Tip: Use /shop to browse available items",
				},
			}},
		})
	}

	actives, _, passives := groupItems(items)
	var currentItems []*models.EffectItem
	var title string

	// Default to active effects
	currentItems = actives
	title = "📦 Your Inventory - Active Effects"

	if len(currentItems) == 0 {
		currentItems = passives
		title = "📦 Your Inventory - Passive Effects"
	}

	if len(currentItems) == 0 {
		return event.CreateMessage(discord.MessageCreate{
			Content: "No items in your inventory",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	components := []discord.ContainerComponent{
		createInventoryCategories("active"),
		createInventoryItems(currentItems, "active"),
	}

	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Title:       title,
			Description: "Select an item to view details",
			Color:       getColorByType(currentItems[0].Type),
			Footer: &discord.EmbedFooter{
				Text: fmt.Sprintf("Total Items: %d", len(items)),
			},
		}},
		Components: components,
	})
}

func createInventoryCategories(selectedValue string) discord.ContainerComponent {
	return discord.NewActionRow(
		discord.NewStringSelectMenu("/inventory_category", "Select Category",
			discord.StringSelectMenuOption{
				Label:       "Active Effects",
				Value:       "active",
				Description: "View your active effect items",
				Emoji:       &discord.ComponentEmoji{Name: "⚔️"},
				Default:     selectedValue == "active",
			},
			discord.StringSelectMenuOption{
				Label:       "Passive Effects",
				Value:       "passive",
				Description: "View your passive effect items",
				Emoji:       &discord.ComponentEmoji{Name: "🛡️"},
				Default:     selectedValue == "passive",
			},
		),
	)
}

func createInventoryItems(items []*models.EffectItem, _ string) discord.ContainerComponent {
	options := make([]discord.StringSelectMenuOption, 0, len(items))
	for _, item := range items {
		options = append(options, discord.StringSelectMenuOption{
			Label:       fmt.Sprintf("%s", item.Name),
			Value:       fmt.Sprintf("inv_%s", item.ID),
			Description: fmt.Sprintf("%dh duration", item.Duration),
			Emoji:       &discord.ComponentEmoji{Name: getTypeEmoji(item.Type)},
		})
	}

	return discord.NewActionRow(
		discord.NewStringSelectMenu("/inventory_item", "Select Item", options...).
			WithMinValues(1).
			WithMaxValues(1),
	)
}

func (h *InventoryHandler) handleItemSelect(event *handler.ComponentEvent) error {
	data, ok := event.Data.(discord.StringSelectMenuInteractionData)
	if !ok || len(data.Values) == 0 {
		return event.CreateMessage(discord.MessageCreate{
			Content: "Invalid interaction data",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	ctx := context.Background()
	userID := event.User().ID.String()
	itemID := strings.TrimPrefix(data.Values[0], "inv_")

	item, err := h.effectManager.GetEffectItem(ctx, itemID)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Failed to fetch item details: %v", err),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	embed := discord.NewEmbedBuilder().
		SetTitle(fmt.Sprintf("%s %s", getTypeEmoji(item.Type), item.Name)).
		SetColor(getColorByType(item.Type))

	// Build description sections
	var description strings.Builder
	description.WriteString("```ansi\n")
	description.WriteString("\u001b[1;33m📋 Item Details\u001b[0m\n")
	description.WriteString(fmt.Sprintf("• Description: %s\n", item.Description))
	description.WriteString(fmt.Sprintf("• Duration: %d hours\n\n", item.Duration))

	if len(item.Recipe) > 0 {
		description.WriteString("\u001b[1;36m🔮 Recipe Requirements\u001b[0m\n")
		cards, err := h.effectManager.GetUserRecipeStatus(ctx, userID, itemID)
		if err != nil {
			log.Printf("[ERROR] Failed to get recipe status: %v", err)
			description.WriteString("Failed to load recipe requirements\n")
		} else {
			for i, card := range cards {
				if card == nil {
					continue
				}

				userCard, err := h.bot.UserCardRepository.GetByUserIDAndCardID(ctx, userID, card.ID)
				hasCard := err == nil && userCard != nil && userCard.Amount > 0

				// Format card name properly
				cardName := strings.Title(strings.ReplaceAll(card.Name, "_", " "))

				if hasCard {
					description.WriteString(fmt.Sprintf("\u001b[1;32m✓ %s ⭐%s\u001b[0m\n",
						cardName, strings.Repeat("", int(item.Recipe[i]))))
				} else {
					description.WriteString(fmt.Sprintf("\u001b[1;31m✗ %s ⭐%s\u001b[0m\n",
						cardName, strings.Repeat("", int(item.Recipe[i]))))
				}
			}
		}
	}
	description.WriteString("```")

	embed.SetDescription(description.String())
	embed.SetFooter("💡 Use /shop to purchase more items", "")

	actionRow := discord.NewActionRow(
		discord.NewSecondaryButton("Back to Inventory ↩️", "/inventory_category"),
	)

	return event.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed.Build()},
		Components: &[]discord.ContainerComponent{actionRow},
	})
}

func (h *InventoryHandler) HandleComponent(event *handler.ComponentEvent) error {
	customID := event.Data.CustomID()

	switch {
	case customID == "/inventory_category":
		return h.handleCategorySelect(event)
	case strings.HasPrefix(customID, "/inventory_item"):
		return h.handleItemSelect(event)
	default:
		return nil
	}
}

func (h *InventoryHandler) handleCategorySelect(event *handler.ComponentEvent) error {
	var selectedValue string

	// Handle both button and select menu interactions
	switch data := event.Data.(type) {
	case discord.StringSelectMenuInteractionData:
		if len(data.Values) == 0 {
			return event.CreateMessage(discord.MessageCreate{
				Content: "Invalid selection",
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
	userID := event.User().ID.String()

	items, err := h.effectManager.ListUserEffects(ctx, userID)
	if err != nil {
		return utils.EH.CreateEphemeralError(event, fmt.Sprintf("Failed to fetch inventory: %v", err))
	}

	actives, _, passives := groupItems(items)
	var currentItems []*models.EffectItem
	var title string

	switch selectedValue {
	case "active":
		currentItems = actives
		title = "📦 Your Inventory - Active Effects"
	case "passive":
		currentItems = passives
		title = "📦 Your Inventory - Passive Effects"
	}

	if len(currentItems) == 0 {
		return event.CreateMessage(discord.MessageCreate{
			Content: "No items in this category",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	components := []discord.ContainerComponent{
		createInventoryCategories(selectedValue),
		createInventoryItems(currentItems, selectedValue),
	}

	return event.UpdateMessage(discord.MessageUpdate{
		Embeds: &[]discord.Embed{{
			Title:       title,
			Description: "Select an item to view details",
			Color:       getColorByType(currentItems[0].Type),
			Footer: &discord.EmbedFooter{
				Text: fmt.Sprintf("Total Items: %d", len(items)),
			},
		}},
		Components: &components,
	})
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

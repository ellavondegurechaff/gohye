package commands

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy/effects"
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
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Failed to fetch inventory: %v", err),
			Flags:   discord.MessageFlagEphemeral,
		})
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

func createInventoryItems(items []*models.EffectItem, itemType string) discord.ContainerComponent {
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

	var recipeInfo strings.Builder
	if len(item.Recipe) > 0 {
		log.Printf("[DEBUG] Processing recipe requirements for item %s", itemID)

		for i, stars := range item.Recipe {
			log.Printf("[DEBUG] Processing recipe requirement %d/%d - %d stars", i+1, len(item.Recipe), stars)

			card, err := h.effectManager.GetRandomCardForRecipe(ctx, userID, stars)
			if err != nil {
				log.Printf("[ERROR] Failed to get recipe card: %v", err)
				recipeInfo.WriteString(fmt.Sprintf("* ❌ %s-star card (%s)\n",
					strings.Repeat("⭐", int(stars)),
					err.Error()))
				continue
			}

			if card == nil {
				log.Printf("[DEBUG] No card returned for %d stars", stars)
				recipeInfo.WriteString(fmt.Sprintf("* ❌ %s-star card (No available cards)\n",
					strings.Repeat("⭐", int(stars))))
				continue
			}

			userCard, err := h.bot.UserCardRepository.GetByUserIDAndCardID(ctx, userID, card.ID)
			if err != nil {
				log.Printf("[ERROR] Failed to check card ownership: %v", err)
			}

			if err != nil || userCard == nil {
				log.Printf("[DEBUG] User %s does not own card %d (%s)", userID, card.ID, card.Name)
				recipeInfo.WriteString(fmt.Sprintf("* ❌ %s-star %s (Need to collect)\n",
					strings.Repeat("⭐", int(stars)), card.Name))
			} else {
				log.Printf("[DEBUG] User %s owns card %d (%s)", userID, card.ID, card.Name)
				recipeInfo.WriteString(fmt.Sprintf("* ✅ %s-star %s (Have)\n",
					strings.Repeat("⭐", int(stars)), card.Name))
			}
		}
	}

	embed := discord.NewEmbedBuilder().
		SetTitle(fmt.Sprintf("%s %s", getTypeEmoji(item.Type), item.Name)).
		SetColor(getColorByType(item.Type)).
		SetDescription(fmt.Sprintf("```md\n## Item Details\n* Description: %s\n* Duration: %d hours\n\n## Recipe Requirements\n%s```",
			item.Description,
			item.Duration,
			recipeInfo.String())).
		SetFooter("💡 Tip: Use /shop to purchase more items", "").
		Build()

	actionRow := discord.NewActionRow(
		discord.NewSecondaryButton("Back to Inventory ↩️", "/inventory_category"),
	)

	return event.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
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
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Failed to fetch inventory: %v", err),
			Flags:   discord.MessageFlagEphemeral,
		})
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

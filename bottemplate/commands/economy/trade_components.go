package economy

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"
)

func (h *TradeHandler) HandleTradeAccept(event *handler.ComponentEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
	defer cancel()

	// Extract trade ID from component custom ID
	parts := strings.Split(event.Data.CustomID(), "/")
	if len(parts) < 4 {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ Invalid trade interaction.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	tradeIDStr := parts[3]
	tradeID, err := strconv.ParseInt(tradeIDStr, 10, 64)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ Invalid trade ID.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Get trade details
	trade, err := h.tradeRepo.GetTradeWithCards(ctx, tradeID)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ Trade not found.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Verify user is the target of this trade
	userID := event.User().ID.String()
	if trade.TargetID != userID {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ You are not authorized to accept this trade.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Check if trade is still pending
	if trade.Status != models.TradePending {
		status := "already processed"
		switch trade.Status {
		case models.TradeAccepted:
			status = "already accepted"
		case models.TradeDeclined:
			status = "already declined"
		case models.TradeExpired:
			status = "expired"
		}
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("❌ This trade is %s.", status),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Execute the trade
	err = h.tradeRepo.ExecuteTrade(ctx, tradeID)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("❌ Failed to execute trade: %s", err.Error()),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Create success embed
	embed := discord.NewEmbedBuilder().
		SetTitle("✅ Trade Completed!").
		SetDescription("The trade has been successfully executed.").
		AddField("You Received", fmt.Sprintf("%s %s", utils.GetPromoRarityPlainText(trade.OffererCard.ColID, trade.OffererCard.Level), trade.OffererCard.Name), false).
		AddField("You Gave", fmt.Sprintf("%s %s", utils.GetPromoRarityPlainText(trade.TargetCard.ColID, trade.TargetCard.Level), trade.TargetCard.Name), false).
		AddField("Trade ID", trade.TradeID, false).
		SetColor(0x00FF00).
		Build()

	// Send DM to offerer about trade completion
	go h.sendTradeCompletionDM(trade.OffererID, trade, true)

	// Update the original message to remove interaction buttons
	return event.UpdateMessage(discord.MessageUpdate{
		Embeds: &[]discord.Embed{embed},
		Components: &[]discord.ContainerComponent{},
	})
}

func (h *TradeHandler) HandleTradeDecline(event *handler.ComponentEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
	defer cancel()

	// Extract trade ID from component custom ID
	parts := strings.Split(event.Data.CustomID(), "/")
	if len(parts) < 4 {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ Invalid trade interaction.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	tradeIDStr := parts[3]
	tradeID, err := strconv.ParseInt(tradeIDStr, 10, 64)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ Invalid trade ID.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Get trade details
	trade, err := h.tradeRepo.GetTradeWithCards(ctx, tradeID)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ Trade not found.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Verify user is the target of this trade
	userID := event.User().ID.String()
	if trade.TargetID != userID {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ You are not authorized to decline this trade.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Check if trade is still pending
	if trade.Status != models.TradePending {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ This trade has already been processed.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Update trade status to declined
	err = h.tradeRepo.UpdateStatus(ctx, tradeID, models.TradeDeclined)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ Failed to decline trade.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Create decline embed
	embed := discord.NewEmbedBuilder().
		SetTitle("❌ Trade Declined").
		SetDescription("You have declined this trade offer.").
		AddField("Trade ID", trade.TradeID, false).
		SetColor(0xFF0000).
		Build()

	// Send DM to offerer about trade decline
	go h.sendTradeCompletionDM(trade.OffererID, trade, false)

	// Update the original message to remove interaction buttons
	return event.UpdateMessage(discord.MessageUpdate{
		Embeds: &[]discord.Embed{embed},
		Components: &[]discord.ContainerComponent{},
	})
}

func (h *TradeHandler) sendTradeCompletionDM(offererID string, trade *models.Trade, accepted bool) {
	dmChannel, err := h.client.Rest().CreateDMChannel(snowflake.MustParse(offererID))
	if err != nil {
		return // Silently fail DM sending
	}

	var embed discord.Embed
	if accepted {
		embed = discord.NewEmbedBuilder().
			SetTitle("✅ Trade Accepted!").
			SetDescription("Your trade offer has been accepted!").
			AddField("You Gave", fmt.Sprintf("%s %s", utils.GetPromoRarityPlainText(trade.OffererCard.ColID, trade.OffererCard.Level), trade.OffererCard.Name), false).
			AddField("You Received", fmt.Sprintf("%s %s", utils.GetPromoRarityPlainText(trade.TargetCard.ColID, trade.TargetCard.Level), trade.TargetCard.Name), false).
			AddField("Trade ID", trade.TradeID, false).
			SetColor(0x00FF00).
			Build()
	} else {
		embed = discord.NewEmbedBuilder().
			SetTitle("❌ Trade Declined").
			SetDescription("Your trade offer has been declined.").
			AddField("Trade ID", trade.TradeID, false).
			SetColor(0xFF0000).
			Build()
	}

	_, _ = h.client.Rest().CreateMessage(dmChannel.ID(), discord.MessageCreate{
		Embeds: []discord.Embed{embed},
	})
}

func (h *TradeHandler) HandleTradeCancel(event *handler.ComponentEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
	defer cancel()

	// Extract trade ID from component custom ID
	parts := strings.Split(event.Data.CustomID(), "/")
	if len(parts) < 4 {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ Invalid trade interaction.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	tradeIDStr := parts[3]
	tradeID, err := strconv.ParseInt(tradeIDStr, 10, 64)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ Invalid trade ID.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Get trade details
	trade, err := h.tradeRepo.GetTradeWithCards(ctx, tradeID)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ Trade not found.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Verify user is the offerer of this trade
	userID := event.User().ID.String()
	if trade.OffererID != userID {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ You can only cancel trades you initiated.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Check if trade is still pending
	if trade.Status != models.TradePending {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ This trade has already been processed and cannot be cancelled.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Cancel the trade by updating status
	err = h.tradeRepo.UpdateStatus(ctx, tradeID, models.TradeDeclined)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ Failed to cancel trade.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Create cancellation embed
	embed := discord.NewEmbedBuilder().
		SetTitle("🚫 Trade Cancelled").
		SetDescription("You have cancelled this trade offer.").
		AddField("Trade ID", trade.TradeID, false).
		SetColor(0xFF6600).
		Build()

	// Send DM to target about trade cancellation
	go h.sendTradeCancellationDM(trade.TargetID, trade)

	// Update the original message to remove interaction buttons
	return event.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &[]discord.ContainerComponent{},
	})
}

func (h *TradeHandler) sendTradeCancellationDM(targetID string, trade *models.Trade) {
	dmChannel, err := h.client.Rest().CreateDMChannel(snowflake.MustParse(targetID))
	if err != nil {
		return // Silently fail DM sending
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("🚫 Trade Cancelled").
		SetDescription("A trade offer sent to you has been cancelled by the sender.").
		AddField("Trade ID", trade.TradeID, false).
		SetColor(0xFF6600).
		Build()

	_, _ = h.client.Rest().CreateMessage(dmChannel.ID(), discord.MessageCreate{
		Embeds: []discord.Embed{embed},
	})
}

// CreateInboxComponentHandler creates component handler for trade inbox pagination
func (h *TradeHandler) CreateInboxComponentHandler() handler.ComponentHandler {
	return func(e *handler.ComponentEvent) error {
		data := e.Data.(discord.ButtonInteractionData)
		customID := data.CustomID()

		// Parse component ID
		parser := utils.NewRegularParser("trade-inbox")
		params, err := parser.Parse(customID)
		if err != nil {
			return nil // Invalid component ID, ignore
		}

		// Validate user
		validator := &InboxValidator{}
		if !validator.ValidateUser(e.User().ID.String(), params) {
			return utils.EH.CreateEphemeralError(e, "Only the command user can navigate through these items.")
		}

		// Handle pagination
		return h.handleInboxPagination(e, params, customID)
	}
}

// handleInboxPagination handles pagination for trade inbox with custom components
func (h *TradeHandler) handleInboxPagination(e *handler.ComponentEvent, params utils.PaginationParams, customID string) error {
	ctx := context.Background()

	// Create data fetcher
	fetcher := &InboxDataFetcher{
		tradeRepo:   h.tradeRepo,
		cardRepo:    h.cardRepo,
		userRepo:    h.userRepo,
		onlyPending: true,
	}

	// Fetch all data
	allItems, err := fetcher.FetchData(ctx, params)
	if err != nil {
		return utils.EH.CreateEphemeralError(e, "Failed to fetch trades")
	}

	if len(allItems) == 0 {
		return utils.EH.CreateEphemeralError(e, "No trades found")
	}

	// Calculate pagination
	itemsPerPage := 5
	totalPages := (len(allItems) + itemsPerPage - 1) / itemsPerPage
	
	// Handle pagination action based on customID
	newPage := params.Page
	if strings.Contains(customID, "/prev/") {
		if newPage > 0 {
			newPage--
		}
	} else if strings.Contains(customID, "/next/") {
		if newPage < totalPages-1 {
			newPage++
		}
	} else if strings.Contains(customID, "/copy/") {
		// Handle copy action
		formatter := &InboxFormatter{userID: params.UserID}
		copyText := formatter.FormatCopy(allItems, params)
		return utils.EH.CreateEphemeralInfo(e, fmt.Sprintf("```\n%s\n```", copyText))
	}

	// Update params with new page
	newParams := params
	newParams.Page = newPage

	// Calculate page items
	startIdx := newPage * itemsPerPage
	endIdx := min(startIdx+itemsPerPage, len(allItems))
	pageItems := allItems[startIdx:endIdx]

	// Create embed
	formatter := &InboxFormatter{userID: params.UserID}
	embed, err := formatter.FormatItems(pageItems, newPage, totalPages, newParams)
	if err != nil {
		return utils.EH.CreateEphemeralError(e, "Failed to format items")
	}

	// Create components with both pagination and trade action buttons
	components := h.createInboxComponents(pageItems, totalPages, newParams)

	return e.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &components,
	})
}

// createInboxComponents creates both pagination and trade action components
func (h *TradeHandler) createInboxComponents(pageItems []interface{}, totalPages int, params utils.PaginationParams) []discord.ContainerComponent {
	var components []discord.ContainerComponent

	// Add pagination buttons if needed
	if totalPages > 1 {
		var paginationButtons []discord.InteractiveComponent

		// Previous button
		parser := utils.NewRegularParser("trade-inbox")
		prevID := parser.BuildComponentID("trade-inbox", "prev", params)
		paginationButtons = append(paginationButtons, discord.NewSecondaryButton("◀ Previous", prevID))

		// Next button
		nextID := parser.BuildComponentID("trade-inbox", "next", params)
		paginationButtons = append(paginationButtons, discord.NewSecondaryButton("Next ▶", nextID))

		// Copy button
		copyID := parser.BuildComponentID("trade-inbox", "copy", params)
		paginationButtons = append(paginationButtons, discord.NewSecondaryButton("📋 Copy Page", copyID))

		components = append(components, discord.NewActionRow(paginationButtons...))
	}

	// Add trade action buttons for pending incoming trades
	for _, item := range pageItems {
		inboxItem := item.(InboxItem)
		trade := inboxItem.Trade

		// Show appropriate buttons based on user role
		if trade.Status == models.TradePending {
			if trade.TargetID == params.UserID {
				// User is the target - show Accept/Decline buttons
				components = append(components, discord.NewActionRow(
					discord.NewSuccessButton(
						fmt.Sprintf("Accept %s", trade.TradeID),
						fmt.Sprintf("/trade/accept/%d", trade.ID),
					),
					discord.NewDangerButton(
						fmt.Sprintf("Decline %s", trade.TradeID),
						fmt.Sprintf("/trade/decline/%d", trade.ID),
					),
				))
			} else if trade.OffererID == params.UserID {
				// User is the offerer - show Cancel button
				components = append(components, discord.NewActionRow(
					discord.NewDangerButton(
						fmt.Sprintf("Cancel %s", trade.TradeID),
						fmt.Sprintf("/trade/cancel/%d", trade.ID),
					),
				))
			}
		}
	}

	return components
}

// InboxItem represents a trade item for pagination
type InboxItem struct {
	Trade       *models.Trade
	OffererCard *models.Card
	TargetCard  *models.Card
	OffererUser *models.User
	TargetUser  *models.User
}

// InboxDataFetcher implements DataFetcher for trade inbox
type InboxDataFetcher struct {
	tradeRepo   repositories.TradeRepository
	cardRepo    repositories.CardRepository
	userRepo    repositories.UserRepository
	onlyPending bool
}

func (f *InboxDataFetcher) FetchData(ctx context.Context, params utils.PaginationParams) ([]interface{}, error) {
	var trades []*models.Trade
	var err error

	if f.onlyPending {
		trades, err = f.tradeRepo.GetUserTrades(ctx, params.UserID, models.TradePending)
	} else {
		trades, err = f.tradeRepo.GetAllUserTrades(ctx, params.UserID)
	}

	if err != nil {
		return nil, err
	}

	var items []interface{}
	for _, trade := range trades {
		// Get card details
		offererCard, err := f.cardRepo.GetByID(ctx, trade.OffererCardID)
		if err != nil {
			continue
		}

		targetCard, err := f.cardRepo.GetByID(ctx, trade.TargetCardID)
		if err != nil {
			continue
		}

		// Get user details
		offererUser, err := f.userRepo.GetByDiscordID(ctx, trade.OffererID)
		if err != nil {
			continue
		}

		targetUser, err := f.userRepo.GetByDiscordID(ctx, trade.TargetID)
		if err != nil {
			continue
		}

		items = append(items, InboxItem{
			Trade:       trade,
			OffererCard: offererCard,
			TargetCard:  targetCard,
			OffererUser: offererUser,
			TargetUser:  targetUser,
		})
	}

	return items, nil
}

// InboxFormatter implements ItemFormatter for trade inbox
type InboxFormatter struct {
	userID string
}

func (f *InboxFormatter) FormatItems(allItems []interface{}, page, totalPages int, params utils.PaginationParams) (discord.Embed, error) {
	// Calculate pagination indices
	itemsPerPage := 5
	startIdx := page * itemsPerPage
	endIdx := min(startIdx+itemsPerPage, len(allItems))

	// Get items for this page only
	pageItems := allItems[startIdx:endIdx]

	var description strings.Builder
	
	for i, item := range pageItems {
		inboxItem := item.(InboxItem)
		trade := inboxItem.Trade
		
		// Determine if this is an incoming or outgoing trade
		isIncoming := trade.TargetID == params.UserID
		var otherUser *models.User
		var youOffer, theyOffer *models.Card
		
		if isIncoming {
			otherUser = inboxItem.OffererUser
			youOffer = inboxItem.TargetCard
			theyOffer = inboxItem.OffererCard
		} else {
			otherUser = inboxItem.TargetUser
			youOffer = inboxItem.OffererCard
			theyOffer = inboxItem.TargetCard
		}

		// Format trade status
		statusEmoji := "⏳"
		statusText := "Pending"
		switch trade.Status {
		case models.TradeAccepted:
			statusEmoji = "✅"
			statusText = "Accepted"
		case models.TradeDeclined:
			statusEmoji = "❌"
			statusText = "Declined"
		case models.TradeExpired:
			statusEmoji = "⏰"
			statusText = "Expired"
		}

		direction := "←"
		if !isIncoming {
			direction = "→"
		}

		description.WriteString(fmt.Sprintf("**%s %s** | %s %s\n", statusEmoji, statusText, direction, otherUser.Username))
		description.WriteString(fmt.Sprintf("You: %s %s\n", utils.GetPromoRarityPlainText(youOffer.ColID, youOffer.Level), youOffer.Name))
		description.WriteString(fmt.Sprintf("Them: %s %s\n", utils.GetPromoRarityPlainText(theyOffer.ColID, theyOffer.Level), theyOffer.Name))
		description.WriteString(fmt.Sprintf("ID: `%s`\n", trade.TradeID))
		
		if i < len(pageItems)-1 {
			description.WriteString("\n")
		}
	}

	// Determine footer text based on user role for pending trades
	footerText := "No pending actions available"
	for _, item := range pageItems {
		inboxItem := item.(InboxItem)
		trade := inboxItem.Trade
		if trade.Status == models.TradePending {
			if trade.TargetID == params.UserID {
				footerText = "Use the buttons below to accept or decline pending trades"
				break
			} else if trade.OffererID == params.UserID {
				footerText = "Use the buttons below to cancel your pending trades"
				break
			}
		}
	}

	embed := discord.Embed{
		Title:       fmt.Sprintf("📥 Trade Inbox - Page %d/%d", page+1, totalPages),
		Description: description.String(),
		Color:       config.BackgroundColor,
		Footer: &discord.EmbedFooter{
			Text: footerText,
		},
	}

	return embed, nil
}

func (f *InboxFormatter) FormatCopy(items []interface{}, params utils.PaginationParams) string {
	var result []string
	for _, item := range items {
		inboxItem := item.(InboxItem)
		trade := inboxItem.Trade
		result = append(result, fmt.Sprintf("%s: %s ↔ %s", trade.TradeID, inboxItem.OffererCard.Name, inboxItem.TargetCard.Name))
	}
	return strings.Join(result, "\n")
}

// InboxValidator implements UserValidator for trade inbox
type InboxValidator struct{}

func (v *InboxValidator) ValidateUser(eventUserID string, params utils.PaginationParams) bool {
	return eventUserID == params.UserID
}
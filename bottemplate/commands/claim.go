package commands

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/uptrace/bun"
)

var Claim = discord.SlashCommandCreate{
	Name:        "claim",
	Description: "✨ Claim cards from the collection!",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionInt{
			Name:        "count",
			Description: "Number of cards to claim (1-10)",
			Required:    false,
			MinValue:    utils.Ptr(1),
			MaxValue:    utils.Ptr(10),
		},
		discord.ApplicationCommandOptionString{
			Name:        "group_type",
			Description: "Type of group to claim from",
			Required:    false,
			Choices: []discord.ApplicationCommandOptionChoiceString{
				{Name: "Girl Groups", Value: "girlgroups"},
				{Name: "Boy Groups", Value: "boygroups"},
			},
		},
	},
}

func getCardImageURL(card *models.Card, bot *bottemplate.Bot) string {
	// Default to girlgroups if no tags or invalid tag
	groupType := "girlgroups"
	if len(card.Tags) > 0 && card.Tags[0] == "boygroups" {
		groupType = "boygroups"
	}

	cardInfo := utils.GetCardDisplayInfo(
		card.Name,
		card.ColID,
		card.Level,
		groupType,
		utils.SpacesConfig{
			Bucket:   bot.SpacesService.GetBucket(),
			Region:   bot.SpacesService.GetRegion(),
			CardRoot: bot.SpacesService.GetCardRoot(),
			GetImageURL: func(cardName string, colID string, level int, groupType string) string {
				return bot.SpacesService.GetCardImageURL(cardName, colID, level, groupType)
			},
		},
	)
	return cardInfo.ImageURL
}

type ClaimHandler struct {
	bot *bottemplate.Bot
}

func NewClaimHandler(b *bottemplate.Bot) *ClaimHandler {
	return &ClaimHandler{
		bot: b,
	}
}

func (h *ClaimHandler) HandleCommand(e *handler.CommandEvent) error {
	ctx := context.Background()
	userID := e.User().ID.String()

	// Get current claim info
	claimInfo, err := h.bot.ClaimRepository.GetClaimInfo(ctx, userID)
	if err != nil {
		return utils.EH.CreateError(e, "Error", "Failed to get claim info")
	}

	// Get options
	count := 1 // Default to 1 if not specified
	if countOption, ok := e.SlashCommandInteractionData().OptInt("count"); ok {
		count = int(countOption)
	}

	// Calculate total cost for all claims
	basePrice := h.bot.ClaimRepository.GetBasePrice()
	currentClaimCount, err := h.bot.ClaimRepository.GetUserClaimsInPeriod(ctx, userID, time.Now().Add(-24*time.Hour))
	if err != nil {
		return utils.EH.CreateError(e, "Error", "Failed to get claim count")
	}

	var totalCost int64
	for i := 0; i < count; i++ {
		claimCost := basePrice * (int64(currentClaimCount) + int64(i) + 1)
		totalCost += claimCost
	}

	// Verify user has enough balance for all claims
	if claimInfo.Balance < totalCost {
		return utils.EH.CreateError(e, "Error", fmt.Sprintf("Insufficient balance. You need %d ⭐ for %d claims", totalCost, count))
	}

	// Start transaction
	tx, err := h.bot.DB.BunDB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Get cards
	cards, err := h.bot.CardRepository.GetAll(context.Background())
	if err != nil {
		return utils.EH.CreateError(e, "Error", "Failed to fetch cards")
	}

	// Filter and select cards
	var selectedCards []*models.Card
	for i := 0; i < count; i++ {
		card := selectRandomCard(cards)
		if card != nil {
			selectedCards = append(selectedCards, card)
		}
	}

	if len(selectedCards) == 0 {
		return utils.EH.CreateError(e, "Error", "No cards available")
	}

	// Create the initial embed
	var cardList strings.Builder
	cardList.WriteString("**✨ New Cards**\n")

	// Show all cards in list
	for _, card := range selectedCards {
		stars := utils.GetStarsDisplay(card.Level)
		collection := fmt.Sprintf("`%s`", strings.ToLower(card.ColID))
		cardList.WriteString(fmt.Sprintf("%s [%s](%s) %s\n",
			stars,
			utils.FormatCardName(card.Name),
			getCardImageURL(card, h.bot),
			collection))
	}

	// Process claims within the transaction
	for _, card := range selectedCards {
		if err := claimCard(ctx, h.bot, card.ID, userID); err != nil {
			return fmt.Errorf("failed to claim card: %w", err)
		}
	}

	// Get current stats for accurate total spent
	stats, err := h.bot.ClaimRepository.GetClaimStats(ctx, userID)
	if err != nil {
		slog.Error("Failed to get claim stats", "error", err)
		// Use default stats if error occurs
		stats = &models.ClaimStats{
			UserID:      userID,
			TotalSpent:  0,
			TotalClaims: 0,
		}
	}

	// Calculate next claim cost based on the updated claim count AFTER this batch
	nextClaimCost := basePrice * (int64(currentClaimCount) + int64(count) + 1)

	// Create receipt text
	receiptText := fmt.Sprintf("You spent %d ⭐ in total\nYou have %d ⭐ left\nYou can claim %d more cards\nYour next claim will cost %d ⭐",
		stats.TotalSpent,
		claimInfo.Balance-totalCost,
		(claimInfo.Balance-totalCost)/nextClaimCost,
		nextClaimCost)

	// Create initial embed with receipt
	embed := discord.NewEmbedBuilder().
		SetDescription(cardList.String()).
		SetColor(utils.SuccessColor).
		SetImage(getCardImageURL(selectedCards[0], h.bot)).
		SetFooter(fmt.Sprintf("Card 1/%d • Claimed by %s", len(selectedCards), e.User().Username), "").
		AddField("Receipt", receiptText, false)

	// Create navigation buttons
	components := []discord.ContainerComponent{
		discord.NewActionRow(
			discord.NewPrimaryButton("◀ Previous", fmt.Sprintf("/claim/prev/%s/1", userID)),
			discord.NewPrimaryButton("Next ▶", fmt.Sprintf("/claim/next/%s/1", userID)),
		),
	}

	// Commit the transaction after all claims are processed
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return e.CreateMessage(discord.MessageCreate{
		Embeds:     []discord.Embed{embed.Build()},
		Components: components,
	})
}

func (h *ClaimHandler) HandleComponent(e *handler.ComponentEvent) error {
	data := e.Data.(discord.ButtonInteractionData)
	customID := data.CustomID()

	slog.Info("Claim component interaction received",
		slog.String("custom_id", customID),
		slog.String("user_id", e.User().ID.String()))

	// Parse the custom ID to get user ID and current page
	parts := strings.Split(customID, "/")
	if len(parts) != 5 {
		slog.Error("Invalid custom ID format",
			slog.String("custom_id", customID),
			slog.Int("parts_length", len(parts)))
		return nil
	}

	claimerID := parts[3]
	currentPage, err := strconv.Atoi(parts[4])
	if err != nil {
		slog.Error("Failed to parse page number",
			slog.String("page_str", parts[4]),
			slog.String("error", err.Error()))
		return nil
	}

	// Check if the user clicking is the one who claimed
	if e.User().ID.String() != claimerID {
		return e.CreateMessage(discord.MessageCreate{
			Content: "Only the user who claimed these cards can navigate through them.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Get the message
	msg := e.Message

	if len(msg.Embeds) == 0 {
		return nil
	}

	// Parse the footer to get total cards count
	footer := msg.Embeds[0].Footer
	if footer == nil {
		return nil
	}

	// Extract total pages from footer text (format: "Card X/Y • Claimed by username")
	totalPages := 0
	fmt.Sscanf(footer.Text, "Card %d/%d", &currentPage, &totalPages)

	// Calculate new page
	newPage := currentPage
	if strings.HasPrefix(customID, "/claim/next/") {
		newPage = (currentPage % totalPages) + 1
	} else if strings.HasPrefix(customID, "/claim/prev/") {
		newPage = ((currentPage - 2 + totalPages) % totalPages) + 1
	}

	// Get card URLs from description
	var cardURLs []string
	lines := strings.Split(msg.Embeds[0].Description, "\n")
	for _, line := range lines {
		if strings.Contains(line, "](") {
			start := strings.Index(line, "](") + 2
			end := strings.Index(line[start:], ")")
			if end > 0 {
				cardURLs = append(cardURLs, line[start:start+end])
			}
		}
	}

	if len(cardURLs) == 0 || newPage > len(cardURLs) {
		return nil
	}

	// Update embed with new page but keep the receipt field
	embed := msg.Embeds[0]
	embed.Footer.Text = fmt.Sprintf("Card %d/%d • Claimed by %s", newPage, totalPages, e.User().Username)
	embed.Image.URL = cardURLs[newPage-1]
	// Receipt field is preserved as it's part of the embed fields

	// Update components with new page number
	components := []discord.ContainerComponent{
		discord.NewActionRow(
			discord.NewPrimaryButton("◀ Previous", fmt.Sprintf("/claim/prev/%s/%d", claimerID, newPage-1)),
			discord.NewPrimaryButton("Next ▶", fmt.Sprintf("/claim/next/%s/%d", claimerID, newPage-1)),
		),
	}

	// Update message
	return e.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &components,
	})
}

func selectRandomCard(cards []*models.Card) *models.Card {
	// Updated weights to make higher levels much rarer and exclude level 5
	weights := map[int]int{
		1: 70, // Common (70% chance)
		2: 20, // Uncommon (20% chance)
		3: 7,  // Rare (7% chance)
		4: 3,  // Epic (3% chance)
		// Level 5 removed completely
	}

	// Filter out level 5 cards and organize by rarity
	var eligibleCards []*models.Card
	cardsByRarity := make(map[int][]*models.Card)

	for _, card := range cards {
		if card.Level < 5 { // Exclude level 5 cards
			eligibleCards = append(eligibleCards, card)
			cardsByRarity[card.Level] = append(cardsByRarity[card.Level], card)
		}
	}

	if len(eligibleCards) == 0 {
		return nil
	}

	// Calculate total weight for eligible cards
	totalWeight := 0
	for rarity, weight := range weights {
		if len(cardsByRarity[rarity]) > 0 {
			totalWeight += weight
		}
	}

	// Roll for rarity
	roll := rand.Intn(totalWeight)
	currentWeight := 0

	// Select rarity based on weights
	for rarity := 1; rarity <= 4; rarity++ {
		currentWeight += weights[rarity]
		if roll < currentWeight && len(cardsByRarity[rarity]) > 0 {
			cards := cardsByRarity[rarity]
			return cards[rand.Intn(len(cards))]
		}
	}

	// Fallback to a random eligible card if something goes wrong
	return eligibleCards[rand.Intn(len(eligibleCards))]
}

func claimCard(ctx context.Context, b *bottemplate.Bot, cardID int64, userID string) error {
	tx, err := b.DB.BunDB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Get claim cost first
	claimCost, err := b.ClaimRepository.GetClaimCost(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get claim cost: %w", err)
	}

	// Update user balance with correct cost
	result, err := tx.NewUpdate().
		Model((*models.User)(nil)).
		Set("balance = balance - ?", claimCost).
		Where("discord_id = ?", userID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update user balance: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		return fmt.Errorf("failed to update user balance - user not found or insufficient balance")
	}

	// Create claim record
	claim := &models.Claim{
		CardID:    cardID,
		UserID:    userID,
		ClaimedAt: time.Now(),
		Expires:   time.Now().Add(24 * time.Hour),
	}

	_, err = tx.NewInsert().Model(claim).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create claim record: %w", err)
	}

	// Update claim stats
	if err := b.ClaimRepository.UpdateClaimStats(ctx, tx, userID, claimCost, true); err != nil {
		return fmt.Errorf("failed to update claim stats: %w", err)
	}

	// Handle user card update/creation
	if err := updateUserCard(ctx, tx, userID, cardID); err != nil {
		return fmt.Errorf("failed to handle user card: %w", err)
	}

	return tx.Commit()
}

func updateUserCard(ctx context.Context, tx bun.Tx, userID string, cardID int64) error {
	result, err := tx.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("amount = amount + 1").
		Set("updated_at = ?", time.Now()).
		Where("user_id = ? AND card_id = ?", userID, cardID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update card: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		userCard := &models.UserCard{
			UserID:    userID,
			CardID:    cardID,
			Amount:    1,
			Obtained:  time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		_, err = tx.NewInsert().Model(userCard).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create user card: %w", err)
		}
	}

	return nil
}

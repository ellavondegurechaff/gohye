package commands

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math/rand"
	"sort"
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

	// Get how many cards they want to claim
	count := 1
	if countOption, ok := e.SlashCommandInteractionData().OptInt("count"); ok {
		count = int(countOption)
	}
	if count < 1 || count > 10 {
		return utils.EH.CreateError(e, "Error", "Count must be between 1 and 10")
	}

	// How many claims user made so far (today)
	currentDailyClaims, err := h.bot.ClaimRepository.GetUserClaimsInPeriod(ctx, userID, time.Now().Add(-24*time.Hour))
	if err != nil {
		return utils.EH.CreateError(e, "Error", "Failed to get claim count")
	}
	basePrice := h.bot.ClaimRepository.GetBasePrice()

	slog.Info("Initial claim state",
		"current_daily_claims", currentDailyClaims,
		"base_price", basePrice,
		"count", count)

	// Calculate costs for each claim in an arithmetic progression
	// cost of nth claim today = basePrice * n
	var totalCost int64
	var claimCosts []int64
	startN := currentDailyClaims + 1
	endN := currentDailyClaims + count

	for n := startN; n <= endN; n++ {
		cost := basePrice * int64(n)
		claimCosts = append(claimCosts, cost)
		totalCost += cost
	}

	// Check if user can afford all claims
	if claimInfo.Balance < totalCost {
		return utils.EH.CreateError(e, "Error",
			fmt.Sprintf("Insufficient balance. You need %d ❄ for %d claims", totalCost, count))
	}

	// Begin transaction
	tx, err := h.bot.DB.BunDB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Retrieve all cards
	cards, err := h.bot.CardRepository.GetAll(ctx)
	if err != nil {
		return utils.EH.CreateError(e, "Error", "Failed to fetch cards")
	}

	// Randomly pick cards
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

	// Sort by level descending
	sort.Slice(selectedCards, func(i, j int) bool {
		return selectedCards[i].Level > selectedCards[j].Level
	})

	// Build the new-cards listing text
	var cardList strings.Builder
	cardList.WriteString("**✨ New Cards**\n\n")
	for _, card := range selectedCards {
		stars := utils.GetStarsDisplay(card.Level)
		collection := fmt.Sprintf("`[%s]`", strings.ToUpper(card.ColID))

		// Check if user already has it
		var userCard models.UserCard
		hasCard := false
		err := tx.NewSelect().
			Model(&userCard).
			Where("user_id = ? AND card_id = ?", userID, card.ID).
			Scan(ctx)
		if err == nil && userCard.Amount > 0 {
			hasCard = true
		}

		cardList.WriteString(fmt.Sprintf(
			"%s **[%s](%s)** %s%s\n",
			stars,
			utils.FormatCardName(card.Name),
			getCardImageURL(card, h.bot),
			collection,
			func() string {
				if hasCard {
					return " `duplicate`"
				}
				return ""
			}(),
		))
	}
	cardList.WriteString("\n\n")

	// Deduct balance & update claim stats per card
	for i, card := range selectedCards {
		if err := claimCard(ctx, h.bot, card.ID, userID, claimCosts[i], count); err != nil {
			return fmt.Errorf("failed to claim card: %w", err)
		}
	}

	// Get updated info & daily claims
	updatedClaimInfo, err := h.bot.ClaimRepository.GetClaimInfo(ctx, userID)
	if err != nil {
		return utils.EH.CreateError(e, "Error", "Failed to get updated claim info")
	}

	finalDailyClaims := currentDailyClaims + count

	// Next claim cost in arithmetic progression is
	//   cost = basePrice * (finalDailyClaims + 1)
	// For the "Spent" line, let's do the sum-of-claims approach:
	sumOfClaims := (finalDailyClaims * (finalDailyClaims + 1)) / 2
	todaysSpent := int64(sumOfClaims) * basePrice
	nextClaimCost := basePrice * int64(finalDailyClaims+1)

	// Compute how many single-card claims user can still afford
	possibleClaims := 0
	tempBalance := updatedClaimInfo.Balance
	tempDailyClaims := finalDailyClaims

	for {
		tempDailyClaims++
		cost := basePrice * int64(tempDailyClaims)
		if cost <= tempBalance {
			possibleClaims++
			tempBalance -= cost
		} else {
			break
		}
	}

	// Build the final receipt
	receiptText := fmt.Sprintf("```md\n"+
		"# Receipt\n"+
		"• Spent: %d \n"+
		"• Balance: %d \n"+
		"• Remaining Claims: %d\n"+
		"• Next Claim Cost: %d \n"+
		"```",
		todaysSpent,
		updatedClaimInfo.Balance,
		possibleClaims,
		nextClaimCost,
	)

	embed := discord.NewEmbedBuilder().
		SetDescription(cardList.String()).
		SetColor(utils.SuccessColor).
		SetImage(getCardImageURL(selectedCards[0], h.bot)).
		SetFooter(fmt.Sprintf("Card 1/%d • Claimed by %s", len(selectedCards), e.User().Username), "").
		AddField("", receiptText, false)

	components := []discord.ContainerComponent{
		discord.NewActionRow(
			discord.NewSecondaryButton("◀ Previous", fmt.Sprintf("/claim/prev/%s/1", userID)),
			discord.NewSecondaryButton("Next ▶", fmt.Sprintf("/claim/next/%s/1", userID)),
		),
	}

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

	// Only the claimer can navigate
	if e.User().ID.String() != claimerID {
		return e.CreateMessage(discord.MessageCreate{
			Content: "Only the user who claimed these cards can navigate through them.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	msg := e.Message
	if len(msg.Embeds) == 0 {
		return nil
	}
	footer := msg.Embeds[0].Footer
	if footer == nil {
		return nil
	}

	// e.g. "Card X/Y • Claimed by ..."
	totalPages := 0
	fmt.Sscanf(footer.Text, "Card %d/%d", &currentPage, &totalPages)

	newPage := currentPage
	if strings.HasPrefix(customID, "/claim/next/") {
		newPage = (currentPage % totalPages) + 1
	} else if strings.HasPrefix(customID, "/claim/prev/") {
		newPage = ((currentPage - 2 + totalPages) % totalPages) + 1
	}

	// Extract image URLs from the embed description lines
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

	// Update the embed
	embed := msg.Embeds[0]
	embed.Footer.Text = fmt.Sprintf("Card %d/%d • Claimed by %s", newPage, totalPages, e.User().Username)
	embed.Image.URL = cardURLs[newPage-1]

	components := []discord.ContainerComponent{
		discord.NewActionRow(
			discord.NewSecondaryButton("◀ Previous", fmt.Sprintf("/claim/prev/%s/%d", claimerID, newPage-1)),
			discord.NewSecondaryButton("Next ▶", fmt.Sprintf("/claim/next/%s/%d", claimerID, newPage-1)),
		),
	}

	return e.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &components,
	})
}

func selectRandomCard(cards []*models.Card) *models.Card {
	// Weighted rarities
	weights := map[int]int{
		1: 70, // Common
		2: 20, // Uncommon
		3: 7,  // Rare
		4: 3,  // Epic
		// Excluding level 5 entirely
	}

	var eligibleCards []*models.Card
	cardsByRarity := make(map[int][]*models.Card)
	for _, card := range cards {
		if card.Level < 5 {
			eligibleCards = append(eligibleCards, card)
			cardsByRarity[card.Level] = append(cardsByRarity[card.Level], card)
		}
	}
	if len(eligibleCards) == 0 {
		return nil
	}

	totalWeight := 0
	for rarity, weight := range weights {
		if len(cardsByRarity[rarity]) > 0 {
			totalWeight += weight
		}
	}
	roll := rand.Intn(totalWeight)
	currentWeight := 0

	for rarity := 1; rarity <= 4; rarity++ {
		currentWeight += weights[rarity]
		if roll < currentWeight && len(cardsByRarity[rarity]) > 0 {
			cards := cardsByRarity[rarity]
			return cards[rand.Intn(len(cards))]
		}
	}
	// fallback
	return eligibleCards[rand.Intn(len(eligibleCards))]
}

func claimCard(ctx context.Context, b *bottemplate.Bot, cardID int64, userID string, claimCost int64, totalClaims int) error {
	tx, err := b.DB.BunDB().BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
	})
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Pass 1 for each single-card claim
	if err := b.ClaimRepository.UpdateClaimStats(ctx, tx, userID, claimCost, true, 1); err != nil {
		return fmt.Errorf("failed to update claim stats: %w", err)
	}

	// Deduct from user balance
	result, err := tx.NewUpdate().
		Model((*models.User)(nil)).
		Set("balance = balance - ?", claimCost).
		Where("discord_id = ?", userID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update user balance: %w", err)
	}
	if rowsAffected, err := result.RowsAffected(); err != nil || rowsAffected == 0 {
		return fmt.Errorf("failed to update user balance - user not found or insufficient balance")
	}

	// Upsert the user_card
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

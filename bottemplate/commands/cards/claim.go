package cards

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
	"github.com/disgoorg/bot-template/bottemplate/cardleveling"
	"github.com/disgoorg/bot-template/bottemplate/config"
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

	// Get group type filter (following pattern from diff.go)
	groupType := strings.TrimSpace(e.SlashCommandInteractionData().String("group_type"))

	// How many claims user made so far (today)
	currentDailyClaims, err := h.bot.ClaimRepository.GetUserClaimsInPeriod(ctx, userID, time.Now().Add(-config.DailyPeriod))
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

	// Retrieve all cards and filter out promo cards
	allCards, err := h.bot.CardRepository.GetAll(ctx)
	if err != nil {
		return utils.EH.CreateError(e, "Error", "Failed to fetch cards")
	}

	// Filter out promo, excluded, and limited collection cards
	var cards []*models.Card
	for _, card := range allCards {
		// Check if card's collection is not promo, excluded, or limited
		if colInfo, exists := utils.GetCollectionInfo(card.ColID); exists && !colInfo.IsPromo && !colInfo.IsExcluded && card.ColID != "limited" {
			cards = append(cards, card)
		}
	}

	// Randomly pick cards (with effect modifications)
	type cardWithEXP struct {
		card *models.Card
		exp  int64
	}
	var selectedCardsWithEXP []cardWithEXP
	for i := 0; i < count; i++ {
		// Apply tohrugift effect for first claim of the day
		isFirstClaim := currentDailyClaims == 0 && i == 0
		card := selectRandomCard(cards, h.bot, userID, isFirstClaim, groupType)
		if card != nil {
			// Calculate initial EXP for non-promo, non-fragment cards
			var exp int64
			if colInfo, exists := utils.GetCollectionInfo(card.ColID); exists && !colInfo.IsPromo && !colInfo.IsFragments {
				exp = calculateInitialEXP(card.Level)
			}
			selectedCardsWithEXP = append(selectedCardsWithEXP, cardWithEXP{card: card, exp: exp})
		}
	}

	if len(selectedCardsWithEXP) == 0 {
		return utils.EH.CreateError(e, "Error", "No cards available")
	}

	// Sort by level descending
	sort.Slice(selectedCardsWithEXP, func(i, j int) bool {
		return selectedCardsWithEXP[i].card.Level > selectedCardsWithEXP[j].card.Level
	})

	// Build the new-cards listing text
	var cardList strings.Builder
	cardList.WriteString("**✨ New Cards**\n\n")
	for _, cardWithExp := range selectedCardsWithEXP {
		card := cardWithExp.card
		stars := utils.GetPromoRarityDisplay(card.ColID, card.Level)
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

		// Build EXP display for new cards
		expDisplay := ""
		// Check if card collection is not promo, not fragments, and not level 5
		colInfo, exists := utils.GetCollectionInfo(card.ColID)
		if exists && !colInfo.IsPromo && !colInfo.IsFragments && card.Level < 5 {
			// Show EXP only for non-promo, non-fragment cards below level 5
			if card.Level < 4 {
				expPercent := calculateExpPercentage(cardWithExp.exp, card.Level)
				expDisplay = fmt.Sprintf(" `%d%%`", expPercent)
			} else {
				// Level 4 cards always show 0% (they don't get initial EXP)
				expDisplay = " `0%`"
			}
		}
		// For promo cards, fragments, and level 5 cards, expDisplay remains empty

		// Format: stars [name](url) [collection] exp% #amount
		cardName := utils.FormatCardName(card.Name)
		amountDisplay := ""
		if hasCard {
			amountDisplay = fmt.Sprintf(" `#%d`", userCard.Amount+1)
		}

		// Build the line with proper spacing
		line := fmt.Sprintf("%s **[%s](%s)** %s", stars, cardName, getCardImageURL(card, h.bot), collection)
		if expDisplay != "" {
			line += expDisplay
		}
		if amountDisplay != "" {
			line += amountDisplay
		}
		cardList.WriteString(line + "\n")
	}
	cardList.WriteString("\n\n")

	// Store card IDs for navigation
	cardIDs := make([]int64, len(selectedCardsWithEXP))
	for i, cardWithExp := range selectedCardsWithEXP {
		cardIDs[i] = cardWithExp.card.ID
	}

	// Deduct balance & update claim stats per card
	for i, cardWithExp := range selectedCardsWithEXP {
		if err := claimCard(ctx, h.bot, cardWithExp.card.ID, userID, claimCosts[i], cardWithExp.exp); err != nil {
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

	// Check if the first card is favorited
	firstCardID := cardIDs[0]
	userCard, err := h.bot.UserCardRepository.GetUserCard(ctx, userID, firstCardID)
	isFavorited := false
	if err == nil && userCard != nil {
		isFavorited = userCard.Favorite
	}

	// Store card IDs in footer for navigation
	cardIDsStr := ""
	for i, id := range cardIDs {
		if i > 0 {
			cardIDsStr += ","
		}
		cardIDsStr += fmt.Sprintf("%d", id)
	}

	embed := discord.NewEmbedBuilder().
		SetDescription(cardList.String()).
		SetColor(utils.SuccessColor).
		SetImage(getCardImageURL(selectedCardsWithEXP[0].card, h.bot)).
		SetFooter(fmt.Sprintf("Card 1/%d • Claimed by %s • IDs:%s", len(selectedCardsWithEXP), e.User().Username, cardIDsStr), "").
		AddField("", receiptText, false)

	// Create favorite button with appropriate emoji
	favoriteEmoji := "🤍"
	if isFavorited {
		favoriteEmoji = "❤️"
	}

	components := []discord.ContainerComponent{
		discord.NewActionRow(
			discord.NewSecondaryButton("◀ Previous", fmt.Sprintf("/claim/prev/%s/1", userID)),
			discord.NewSecondaryButton(favoriteEmoji, fmt.Sprintf("/claim/favorite/%s/%d/1", userID, firstCardID)),
			discord.NewSecondaryButton("Next ▶", fmt.Sprintf("/claim/next/%s/1", userID)),
		),
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Check for collection completion after successful claim
	claimedCardIDs := make([]int64, len(selectedCardsWithEXP))
	for i, cardWithExp := range selectedCardsWithEXP {
		claimedCardIDs[i] = cardWithExp.card.ID
	}
	go h.bot.CompletionChecker.CheckCompletionForCards(context.Background(), userID, claimedCardIDs)

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
	if len(parts) < 5 {
		slog.Error("Invalid custom ID format",
			slog.String("custom_id", customID),
			slog.Int("parts_length", len(parts)))
		return nil
	}

	action := parts[2]
	claimerID := parts[3]
	
	// Handle favorite button
	if action == "favorite" {
		if len(parts) != 6 {
			slog.Error("Invalid favorite custom ID format",
				slog.String("custom_id", customID),
				slog.Int("parts_length", len(parts)))
			return nil
		}
		
		cardID, err := strconv.ParseInt(parts[4], 10, 64)
		if err != nil {
			return nil
		}
		
		// Only the claimer can favorite
		if e.User().ID.String() != claimerID {
			return e.CreateMessage(discord.MessageCreate{
				Content: "Only the user who claimed these cards can favorite them.",
				Flags:   discord.MessageFlagEphemeral,
			})
		}
		
		// Toggle favorite status
		ctx := context.Background()
		isFavorited, err := h.bot.UserCardRepository.ToggleFavorite(ctx, claimerID, cardID)
		if err != nil {
			slog.Error("Failed to toggle favorite", 
				slog.String("error", err.Error()),
				slog.String("user_id", claimerID),
				slog.Int64("card_id", cardID))
			return e.CreateMessage(discord.MessageCreate{
				Content: "Failed to toggle favorite status.",
				Flags:   discord.MessageFlagEphemeral,
			})
		}
		
		// Update the button emoji
		msg := e.Message
		if len(msg.Components) > 0 {
			actionRow := msg.Components[0].(discord.ActionRowComponent)
			buttons := actionRow.Components()
			
			// Update the middle button (favorite button)
			if len(buttons) >= 3 {
				favoriteEmoji := "🤍"
				if isFavorited {
					favoriteEmoji = "❤️"
				}
				buttons[1] = discord.NewSecondaryButton(favoriteEmoji, customID)
				
				components := []discord.ContainerComponent{
					discord.NewActionRow(buttons...),
				}
				
				return e.UpdateMessage(discord.MessageUpdate{
					Components: &components,
				})
			}
		}
		
		return nil
	}

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

	// Extract card IDs from footer
	// Format: "Card X/Y • Claimed by USERNAME • IDs:1,2,3"
	footerParts := strings.Split(footer.Text, " • ")
	cardIDsStr := ""
	if len(footerParts) >= 3 && strings.HasPrefix(footerParts[2], "IDs:") {
		cardIDsStr = strings.TrimPrefix(footerParts[2], "IDs:")
	}
	
	// Parse card IDs
	var cardIDs []int64
	if cardIDsStr != "" {
		idStrs := strings.Split(cardIDsStr, ",")
		for _, idStr := range idStrs {
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err == nil {
				cardIDs = append(cardIDs, id)
			}
		}
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
	embed.Footer.Text = fmt.Sprintf("Card %d/%d • Claimed by %s • IDs:%s", newPage, totalPages, e.User().Username, cardIDsStr)
	embed.Image.URL = cardURLs[newPage-1]

	// Get the card ID for the current page and check if it's favorited
	currentCardID := int64(0)
	if newPage-1 < len(cardIDs) {
		currentCardID = cardIDs[newPage-1]
	}
	
	// Check if current card is favorited
	ctx := context.Background()
	favoriteEmoji := "🤍"
	if currentCardID > 0 {
		userCard, err := h.bot.UserCardRepository.GetUserCard(ctx, claimerID, currentCardID)
		if err == nil && userCard != nil && userCard.Favorite {
			favoriteEmoji = "❤️"
		}
	}

	components := []discord.ContainerComponent{
		discord.NewActionRow(
			discord.NewSecondaryButton("◀ Previous", fmt.Sprintf("/claim/prev/%s/%d", claimerID, newPage-1)),
			discord.NewSecondaryButton(favoriteEmoji, fmt.Sprintf("/claim/favorite/%s/%d/%d", claimerID, currentCardID, newPage)),
			discord.NewSecondaryButton("Next ▶", fmt.Sprintf("/claim/next/%s/%d", claimerID, newPage-1)),
		),
	}

	return e.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &components,
	})
}

// calculateExpPercentage calculates the EXP percentage for display
func calculateExpPercentage(currentExp int64, level int) int {
	if level >= 5 {
		return 100 // Level 5 cards are maxed
	}

	calculator := cardleveling.NewCalculator(cardleveling.NewDefaultConfig())
	requiredExp := calculator.CalculateExpRequirement(level)

	if requiredExp <= 0 {
		return 0
	}

	percentage := int((currentExp * 100) / requiredExp)
	// Ensure at least 1% shows if there's any EXP
	if percentage == 0 && currentExp > 0 {
		percentage = 1
	}
	if percentage > 100 {
		percentage = 100
	}
	return percentage
}

// calculateInitialEXP calculates the initial EXP for a newly claimed card
// All cards get at least some EXP to start with (minimum 1% of required)
func calculateInitialEXP(cardLevel int) int64 {
	// Skip EXP for level 4+ cards
	if cardLevel >= 4 {
		return 0
	}

	// Random multiplier from 0-8 (0 gives minimal EXP, 1-8 gives more)
	multiplier := rand.Intn(9)

	// Calculate EXP based on card level
	// Ensure minimum is at least 1% of required EXP
	var exp int64
	switch cardLevel {
	case 1:
		// Required: 4500, so 1% = 45
		if multiplier == 0 {
			exp = int64(rand.Intn(50) + 45) // 45-94 for minimal EXP (1-2%)
		} else {
			exp = int64(multiplier*125) + 45 // 170-1045
		}
	case 2:
		// Required: 22500 (15000 * 1.5), so 1% = 225
		if multiplier == 0 {
			exp = int64(rand.Intn(225) + 225) // 225-449 for minimal EXP (1-2%)
		} else {
			exp = int64(float64(multiplier)*187.5) + 225 // 412-1725
		}
	case 3:
		// Required: 101250 (45000 * 2.25), so 1% = 1013
		if multiplier == 0 {
			exp = int64(rand.Intn(500) + 1013) // 1013-1512 for minimal EXP (1-1.5%)
		} else {
			exp = int64(multiplier*250) + 1013 // 1263-3013
		}
	default:
		exp = 0
	}

	return exp
}

func selectRandomCard(cards []*models.Card, bot *bottemplate.Bot, userID string, isFirstClaim bool, groupType string) *models.Card {
	// Weighted rarities
	weights := map[int]int{
		1: 70, // Common
		2: 20, // Uncommon
		3: 7,  // Rare
		4: 3,  // Epic
		// Excluding level 5 entirely
	}

	// Apply tohrugift effect for first claim
	if isFirstClaim && bot.EffectIntegrator != nil {
		ctx := context.Background()
		baseChance := float64(weights[3]) // 3-star base chance
		modifiedChance := bot.EffectIntegrator.ApplyClaimEffects(ctx, userID, baseChance)

		if modifiedChance > baseChance {
			// Increase 3-star weight, reduce others proportionally
			weights[3] = int(modifiedChance)
			weights[1] = 65 // Slightly reduce common
			weights[2] = 18 // Slightly reduce uncommon
		}
	}

	var eligibleCards []*models.Card
	cardsByRarity := make(map[int][]*models.Card)
	for _, card := range cards {
		// Skip cards that don't match level requirement
		if card.Level >= 5 {
			continue
		}

		// Apply group type filter (following pattern from search_utils.go:215-239)
		if groupType != "" {
			hasMatchingTag := false
			for _, tag := range card.Tags {
				if tag == groupType {
					hasMatchingTag = true
					break
				}
			}
			if !hasMatchingTag {
				continue
			}
		}

		eligibleCards = append(eligibleCards, card)
		cardsByRarity[card.Level] = append(cardsByRarity[card.Level], card)
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

func claimCard(ctx context.Context, b *bottemplate.Bot, cardID int64, userID string, claimCost int64, initialExp int64) error {
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
	if err := updateUserCard(ctx, tx, userID, cardID, initialExp); err != nil {
		return fmt.Errorf("failed to handle user card: %w", err)
	}

	return tx.Commit()
}

func updateUserCard(ctx context.Context, tx bun.Tx, userID string, cardID int64, initialExp int64) error {
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
		// New card - set initial EXP
		userCard := &models.UserCard{
			UserID:    userID,
			CardID:    cardID,
			Amount:    1,
			Exp:       initialExp,
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

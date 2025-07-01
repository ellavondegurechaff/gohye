package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate/cardleveling"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/interfaces"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
)

// CardDisplayService provides unified card display functionality
type CardDisplayService struct {
	cardRepo      interfaces.CardRepositoryInterface
	spacesService interfaces.SpacesServiceInterface
}

// NewCardDisplayService creates a new card display service
func NewCardDisplayService(cardRepo interfaces.CardRepositoryInterface, spacesService interfaces.SpacesServiceInterface) *CardDisplayService {
	return &CardDisplayService{
		cardRepo:      cardRepo,
		spacesService: spacesService,
	}
}

// CardDisplayItem represents an item that can be displayed in card lists
type CardDisplayItem interface {
	GetCardID() int64
	GetAmount() int
	IsFavorite() bool
	IsAnimated() bool
	GetExtraInfo() []string
}

// UserCardDisplay wraps a UserCard for display
type UserCardDisplay struct {
	UserCard *models.UserCard
	Card     *models.Card
	User     *models.User // Optional user data for new card detection
}

func (ucd *UserCardDisplay) GetCardID() int64 {
	return ucd.UserCard.CardID
}

func (ucd *UserCardDisplay) GetAmount() int {
	return int(ucd.UserCard.Amount)
}

func (ucd *UserCardDisplay) IsFavorite() bool {
	return ucd.UserCard.Favorite
}

func (ucd *UserCardDisplay) IsAnimated() bool {
	return ucd.Card.Animated
}

// IsNewCard returns true if card was obtained after user's last daily
func (ucd *UserCardDisplay) IsNewCard() bool {
	if ucd.User == nil {
		return false
	}
	return ucd.UserCard.Obtained.After(ucd.User.LastDaily)
}

// IsLocked returns true if the card is locked
func (ucd *UserCardDisplay) IsLocked() bool {
	return ucd.UserCard.Locked
}

func (ucd *UserCardDisplay) GetExtraInfo() []string {
	var extras []string

	// Add EXP percentage only for non-promo, non-fragment, non-excluded cards below level 5
	colInfo, exists := utils.GetCollectionInfo(ucd.Card.ColID)
	if exists && !colInfo.IsPromo && !colInfo.IsFragments && !colInfo.IsExcluded && ucd.UserCard.Level < 5 {
		expPercent := calculateExpPercentage(ucd.UserCard.Exp, ucd.UserCard.Level)
		extras = append(extras, fmt.Sprintf("`%d%%`", expPercent))
	}
	// For promo cards, fragments, excluded collections, and level 5 cards, no EXP is shown

	// Add custom mark if exists
	if ucd.UserCard.Mark != "" {
		extras = append(extras, fmt.Sprintf("`%s`", ucd.UserCard.Mark))
	}

	// Add rating for non-promo cards with rating
	if ucd.UserCard.Rating > 0 && !config.IsPromoCollection(ucd.Card.ColID) {
		extras = append(extras, fmt.Sprintf("`(%d⏫)`", ucd.UserCard.Rating))
	}

	return extras
}

// UserCardDisplayWithContext wraps a UserCard for context-aware display based on sorting
type UserCardDisplayWithContext struct {
	UserCard *models.UserCard
	Card     *models.Card
	User     *models.User // Optional user data for new card detection
	Filters  utils.SearchFilters // Search context for display decisions
}

func (ucdc *UserCardDisplayWithContext) GetCardID() int64 {
	return ucdc.UserCard.CardID
}

func (ucdc *UserCardDisplayWithContext) GetAmount() int {
	return int(ucdc.UserCard.Amount)
}

func (ucdc *UserCardDisplayWithContext) IsFavorite() bool {
	return ucdc.UserCard.Favorite
}

func (ucdc *UserCardDisplayWithContext) IsAnimated() bool {
	return ucdc.Card.Animated
}

// IsNewCard returns true if card was obtained after user's last daily
func (ucdc *UserCardDisplayWithContext) IsNewCard() bool {
	if ucdc.User == nil {
		return false
	}
	return ucdc.UserCard.Obtained.After(ucdc.User.LastDaily)
}

// IsLocked returns true if the card is locked
func (ucdc *UserCardDisplayWithContext) IsLocked() bool {
	return ucdc.UserCard.Locked
}

func (ucdc *UserCardDisplayWithContext) GetExtraInfo() []string {
	var extras []string

	// Context-aware display based on sorting criteria
	switch ucdc.Filters.SortBy {
	case utils.SortByExp:
		// Show EXP percentage when sorting by experience (except for promo, fragments, and level 5)
		colInfo, exists := utils.GetCollectionInfo(ucdc.Card.ColID)
		if exists && !colInfo.IsPromo && !colInfo.IsFragments && ucdc.UserCard.Level < 5 {
			expPercent := calculateExpPercentage(ucdc.UserCard.Exp, ucdc.UserCard.Level)
			extras = append(extras, fmt.Sprintf("**`%d%%`**", expPercent))
		}
	case utils.SortByAmount:
		// Prominently show amount when sorting by amount
		if ucdc.UserCard.Amount > 1 {
			extras = append(extras, fmt.Sprintf("**x%d**", ucdc.UserCard.Amount))
		}
	case utils.SortByRating:
		// Show rating when sorting by rating
		if ucdc.UserCard.Rating > 0 {
			extras = append(extras, fmt.Sprintf("**★%d**", ucdc.UserCard.Rating))
		} else {
			extras = append(extras, "**★0**")
		}
	case utils.SortByDate:
		// Show relative date when sorting by date
		if !ucdc.UserCard.Obtained.IsZero() {
			// You could add relative time formatting here
			extras = append(extras, fmt.Sprintf("**%s**", ucdc.UserCard.Obtained.Format("Jan 2")))
		}
	default:
		// Default behavior - show EXP percentage only for non-promo, non-fragment, non-excluded cards below level 5
		colInfo, exists := utils.GetCollectionInfo(ucdc.Card.ColID)
		if exists && !colInfo.IsPromo && !colInfo.IsFragments && !colInfo.IsExcluded && ucdc.UserCard.Level < 5 {
			expPercent := calculateExpPercentage(ucdc.UserCard.Exp, ucdc.UserCard.Level)
			extras = append(extras, fmt.Sprintf("`%d%%`", expPercent))
		}
		// For promo cards, fragments, excluded collections, and level 5 cards, no EXP is shown
		
		// Show amount for multiples in default view
		if ucdc.UserCard.Amount > 1 {
			extras = append(extras, fmt.Sprintf("`x%d`", ucdc.UserCard.Amount))
		}
	}

	// Always add custom mark if exists
	if ucdc.UserCard.Mark != "" {
		extras = append(extras, fmt.Sprintf("`%s`", ucdc.UserCard.Mark))
	}

	return extras
}

// MissingCardDisplay wraps a Card for missing card display
type MissingCardDisplay struct {
	Card *models.Card
}

func (mcd *MissingCardDisplay) GetCardID() int64 {
	return mcd.Card.ID
}

func (mcd *MissingCardDisplay) GetAmount() int {
	return 0
}

func (mcd *MissingCardDisplay) IsFavorite() bool {
	return false
}

func (mcd *MissingCardDisplay) IsAnimated() bool {
	return mcd.Card.Animated
}

func (mcd *MissingCardDisplay) GetExtraInfo() []string {
	return nil
}

// DiffCardDisplay wraps a Card with diff percentage for diff command
type DiffCardDisplay struct {
	Card       *models.Card
	Percentage string
}

func (dcd *DiffCardDisplay) GetCardID() int64 {
	return dcd.Card.ID
}

func (dcd *DiffCardDisplay) GetAmount() int {
	return 0
}

func (dcd *DiffCardDisplay) IsFavorite() bool {
	return false
}

func (dcd *DiffCardDisplay) IsAnimated() bool {
	return dcd.Card.Animated
}

func (dcd *DiffCardDisplay) GetExtraInfo() []string {
	if dcd.Percentage != "" {
		return []string{dcd.Percentage}
	}
	return nil
}

// FormatCardDisplayItems formats a slice of CardDisplayItems into a description string
func (cds *CardDisplayService) FormatCardDisplayItems(ctx context.Context, items []CardDisplayItem) (string, error) {
	var description strings.Builder

	for _, item := range items {
		card, err := cds.cardRepo.GetByID(ctx, item.GetCardID())
		if err != nil {
			continue // Skip cards we can't fetch
		}

		groupType := utils.GetGroupType(card.Tags)
		
		// Always use base card level for star display (Card.Level = star rating 1-5)
		// UserCard.Level is progression level which is different from star rating
		displayLevel := card.Level
		
		displayInfo := utils.GetCardDisplayInfo(
			card.Name,
			card.ColID,
			displayLevel,
			groupType,
			cds.spacesService.GetSpacesConfig(),
		)

		var entry string
		// Check if this is a UserCardDisplay to show new and lock indicators
		if userCardDisplay, ok := item.(*UserCardDisplay); ok {
			entry = utils.FormatCardEntryWithIndicators(
				displayInfo,
				item.IsFavorite(),
				item.IsAnimated(),
				item.GetAmount(),
				userCardDisplay.IsNewCard(),
				userCardDisplay.IsLocked(),
				item.GetExtraInfo()...,
			)
		} else {
			entry = utils.FormatCardEntry(
				displayInfo,
				item.IsFavorite(),
				item.IsAnimated(),
				item.GetAmount(),
				item.GetExtraInfo()...,
			)
		}

		description.WriteString(entry + "\n")
	}

	return description.String(), nil
}

// CreateCardsEmbed creates a standardized cards embed
func (cds *CardDisplayService) CreateCardsEmbed(
	ctx context.Context,
	title string,
	items []CardDisplayItem,
	page, totalPages, totalItems int,
	query string,
	color int,
) (discord.Embed, error) {
	description, err := cds.FormatCardDisplayItems(ctx, items)
	if err != nil {
		return discord.Embed{}, err
	}

	embed := discord.NewEmbedBuilder().
		SetTitle(title).
		SetDescription(description).
		SetColor(color).
		SetFooter(fmt.Sprintf("Page %d/%d • Total: %d", page+1, totalPages, totalItems), "")

	if query != "" {
		embed.SetDescription(fmt.Sprintf("`🔍 %s`\n\n%s", query, description))
	}

	return embed.Build(), nil
}

// FormatCopyText creates copy-friendly text for cards
func (cds *CardDisplayService) FormatCopyText(ctx context.Context, items []CardDisplayItem, title string) (string, error) {
	var sb strings.Builder
	sb.WriteString(title + "\n")

	for _, item := range items {
		card, err := cds.cardRepo.GetByID(ctx, item.GetCardID())
		if err != nil {
			continue
		}

		stars := utils.GetPromoRarityPlainText(card.ColID, card.Level)
		line := fmt.Sprintf("%s %s [%s]", stars, utils.FormatCardName(card.Name), card.ColID)

		if item.GetAmount() > 1 {
			line += fmt.Sprintf(" x%d", item.GetAmount())
		}

		sb.WriteString(line + "\n")
	}

	return sb.String(), nil
}

// ConvertUserCardsToDisplayItems converts UserCard slice to CardDisplayItem slice
func (cds *CardDisplayService) ConvertUserCardsToDisplayItems(ctx context.Context, userCards []*models.UserCard) ([]CardDisplayItem, error) {
	items := make([]CardDisplayItem, 0, len(userCards))

	for _, userCard := range userCards {
		card, err := cds.cardRepo.GetByID(ctx, userCard.CardID)
		if err != nil {
			continue // Skip cards we can't fetch
		}

		items = append(items, &UserCardDisplay{
			UserCard: userCard,
			Card:     card,
		})
	}

	return items, nil
}

// ConvertUserCardsToDisplayItemsWithUser converts UserCard slice to CardDisplayItem slice with User data
func (cds *CardDisplayService) ConvertUserCardsToDisplayItemsWithUser(ctx context.Context, userCards []*models.UserCard, user *models.User) ([]CardDisplayItem, error) {
	items := make([]CardDisplayItem, 0, len(userCards))

	for _, userCard := range userCards {
		card, err := cds.cardRepo.GetByID(ctx, userCard.CardID)
		if err != nil {
			continue // Skip cards we can't fetch
		}

		items = append(items, &UserCardDisplay{
			UserCard: userCard,
			Card:     card,
			User:     user,
		})
	}

	return items, nil
}

// ConvertUserCardsToDisplayItemsWithUserAndContext converts UserCard slice to CardDisplayItem slice with User data and sorting context
func (cds *CardDisplayService) ConvertUserCardsToDisplayItemsWithUserAndContext(ctx context.Context, userCards []*models.UserCard, user *models.User, filters utils.SearchFilters) ([]CardDisplayItem, error) {
	items := make([]CardDisplayItem, 0, len(userCards))

	for _, userCard := range userCards {
		card, err := cds.cardRepo.GetByID(ctx, userCard.CardID)
		if err != nil {
			continue // Skip cards we can't fetch
		}

		items = append(items, &UserCardDisplayWithContext{
			UserCard: userCard,
			Card:     card,
			User:     user,
			Filters:  filters,
		})
	}

	return items, nil
}

// ConvertCardsToMissingDisplayItems converts Card slice to CardDisplayItem slice for missing cards
func (cds *CardDisplayService) ConvertCardsToMissingDisplayItems(cards []*models.Card) []CardDisplayItem {
	items := make([]CardDisplayItem, 0, len(cards))

	for _, card := range cards {
		items = append(items, &MissingCardDisplay{
			Card: card,
		})
	}

	return items
}

// ConvertCardsToDisplayItems converts Card slice to CardDisplayItem slice
func (cds *CardDisplayService) ConvertCardsToDisplayItems(cards []*models.Card) []CardDisplayItem {
	items := make([]CardDisplayItem, len(cards))
	for i, card := range cards {
		items[i] = &MissingCardDisplay{Card: card}
	}
	return items
}

// ConvertCardsToDiffDisplayItems converts Card slice with percentages to CardDisplayItem slice
func (cds *CardDisplayService) ConvertCardsToDiffDisplayItems(cards []*models.Card, percentages []string) []CardDisplayItem {
	items := make([]CardDisplayItem, len(cards))
	for i, card := range cards {
		percentage := ""
		if i < len(percentages) {
			percentage = percentages[i]
		}
		items[i] = &DiffCardDisplay{
			Card:       card,
			Percentage: percentage,
		}
	}
	return items
}

// ConvertCardsToDiffDisplayItemsSimple converts Card slice to CardDisplayItem slice for diff command
func (cds *CardDisplayService) ConvertCardsToDiffDisplayItemsSimple(cards []*models.Card) []CardDisplayItem {
	items := make([]CardDisplayItem, len(cards))
	for i, card := range cards {
		items[i] = &DiffCardDisplay{
			Card:       card,
			Percentage: "",
		}
	}
	return items
}

// LimitedCardDisplay wraps a Card for limited card display
type LimitedCardDisplay struct {
	Card *models.Card
}

func (lcd *LimitedCardDisplay) GetCardID() int64 {
	return lcd.Card.ID
}

func (lcd *LimitedCardDisplay) GetAmount() int {
	return 0
}

func (lcd *LimitedCardDisplay) IsFavorite() bool {
	return false
}

func (lcd *LimitedCardDisplay) IsAnimated() bool {
	return lcd.Card.Animated
}

func (lcd *LimitedCardDisplay) GetExtraInfo() []string {
	return []string{fmt.Sprintf("#%d", lcd.Card.ID)}
}

// ConvertCardsToLimitedDisplayItems converts Card slice to CardDisplayItem slice for limited cards
func (cds *CardDisplayService) ConvertCardsToLimitedDisplayItems(cards []*models.Card) []CardDisplayItem {
	items := make([]CardDisplayItem, len(cards))
	for i, card := range cards {
		items[i] = &LimitedCardDisplay{
			Card: card,
		}
	}
	return items
}

// LimitedStatsDisplay wraps a Card with ownership statistics for limited stats display
type LimitedStatsDisplay struct {
	Card   *models.Card
	Owners int64
}

func (lsd *LimitedStatsDisplay) GetCardID() int64 {
	return lsd.Card.ID
}

func (lsd *LimitedStatsDisplay) GetAmount() int {
	return 0
}

func (lsd *LimitedStatsDisplay) IsFavorite() bool {
	return false
}

func (lsd *LimitedStatsDisplay) IsAnimated() bool {
	return lsd.Card.Animated
}

func (lsd *LimitedStatsDisplay) GetExtraInfo() []string {
	ownerText := "owners"
	if lsd.Owners == 1 {
		ownerText = "owner"
	}
	return []string{fmt.Sprintf("#%d", lsd.Card.ID), fmt.Sprintf("%d %s", lsd.Owners, ownerText)}
}

// ConvertStatsToLimitedStatsDisplayItems converts cardStat slice to CardDisplayItem slice for limited stats
func (cds *CardDisplayService) ConvertStatsToLimitedStatsDisplayItems(stats interface{}) []CardDisplayItem {
	// We need to use interface{} because we can't import the cardStat type here
	// The calling code will pass the stats and we'll handle them generically
	return nil // This will be implemented in the command file directly
}

// CreatePaginatedCardsEmbed creates a paginated cards embed for a specific page
func (cds *CardDisplayService) CreatePaginatedCardsEmbed(
	ctx context.Context,
	title string,
	allItems []CardDisplayItem,
	page int,
	query string,
	color int,
) (discord.Embed, error) {
	itemsPerPage := config.CardsPerPage
	totalPages := (len(allItems) + itemsPerPage - 1) / itemsPerPage

	startIdx := page * itemsPerPage
	endIdx := startIdx + itemsPerPage
	if endIdx > len(allItems) {
		endIdx = len(allItems)
	}

	pageItems := allItems[startIdx:endIdx]

	return cds.CreateCardsEmbed(
		ctx,
		title,
		pageItems,
		page,
		totalPages,
		len(allItems),
		query,
		color,
	)
}

// calculateExpPercentage calculates EXP percentage using our cardleveling system
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
	if percentage > 100 {
		percentage = 100
	}
	if percentage < 0 {
		percentage = 0
	}

	return percentage
}

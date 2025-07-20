// converters.go
package migration

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (m *Migrator) convertUser(mu MongoUser) *models.User {
	now := time.Now()

	var cardID int64
	if mu.LastQueriedCard.ID != "" {
		var err error
		cardID, err = strconv.ParseInt(mu.LastQueriedCard.ID, 10, 64)
		if err != nil {
			cardID = 0
		}
	}

	return &models.User{
		DiscordID:       mu.DiscordID,
		Username:        cleanseString(mu.Username),
		Balance:         int64(mu.Exp),
		PromoExp:        int64(mu.PromoExp),
		Joined:          mu.Joined,
		LastQueriedCard: convertCard(mu.LastQueriedCard, cardID),
		LastKofiClaim:   mu.LastKofiClaim,
		DailyStats:      convertGameStats(mu.DailyStats),
		EffectStats:     convertMemoryStats(mu.EffectUseCount),
		UserStats:       convertCoreStats(mu),
		Cards:           mu.Cards,
		Inventory:       convertInventory(mu.Inventory),
		CompletedCols:   models.FlexibleCompletedCols(convertCompletedCols(mu.CompletedCols)),
		CloutedCols:     models.FlexibleCloutedCols(convertCloutedCols(mu.CloutedCols)),
		Achievements:    mu.Achievements,
		Effects:         mu.Effects,
		Wishlist:        convertWishlist(mu.Wishlist),
		LastDaily:       mu.LastDaily,
		LastTrain:       mu.LastTrain,
		LastWork:        mu.LastWork,
		LastVote:        mu.LastVote,
		LastAnnounce:    mu.LastAnnounce,
		LastMsg:         cleanseString(mu.LastMsg),
		HeroSlots:       mu.HeroSlots,
		HeroCooldown:    mu.HeroCooldown,
		Hero:            mu.Hero,
		HeroChanged:     mu.HeroChanged,
		HeroSubmits:     int(mu.HeroSubmits),
		Roles:           mu.Roles,
		Ban:             convertBanInfo(mu.Ban),
		Premium:         mu.Premium,
		PremiumExpires:  mu.PremiumExpires,
		CreatedAt:       now,
		UpdatedAt:       now,
		Preferences:     convertPreferences(mu.Prefs),
	}
}

func convertCard(mqc LastQueriedCard, cardID int64) models.Card {
	return models.Card{
		ID:       cardID,
		Name:     cleanseString(mqc.Name),
		Level:    int(mqc.Level),
		Animated: mqc.Animated,
		ColID:    mqc.ColID,
		Tags:     mqc.Tags,
	}
}

func convertGameStats(ds DailyStats) models.GameStats {
	return models.GameStats{
		Claims:         int(ds.Claims),
		PromoClaims:    int(ds.PromoClaims),
		TotalRegClaims: int(ds.TotalRegClaims),
		Bids:           int(ds.Bids),
		Aucs:           int(ds.Aucs),
		Liquify:        int(ds.Liquify),
		Liquify1:       int(ds.Liquify1),
		Liquify2:       int(ds.Liquify2),
		Liquify3:       int(ds.Liquify3),
		Draw:           int(ds.Draw),
		Draw1:          int(ds.Draw1),
		Draw2:          int(ds.Draw2),
		Draw3:          int(ds.Draw3),
		Tags:           int(ds.Tags),
		Forge1:         int(ds.Forge1),
		Forge2:         int(ds.Forge2),
		Forge3:         int(ds.Forge3),
		Rates:          int(ds.Rates),
		Store3:         int(ds.Store3),
	}
}

func convertMemoryStats(euc EffectUseCount) models.MemoryStats {
	return models.MemoryStats{
		MemoryXmas: int(euc.MemoryXmas),
		MemoryHall: int(euc.MemoryHall),
		MemoryBday: int(euc.MemoryBday),
		MemoryVal:  int(euc.MemoryVal),
		XmasSpace:  euc.XmasSpace,
		HallSpace:  euc.HallSpace,
		BdaySpace:  euc.BdaySpace,
		ValSpace:   euc.ValSpace,
	}
}

func convertCoreStats(mu MongoUser) models.CoreStats {
	return models.CoreStats{
		LastCard: int64(mu.LastCard),
		XP:       int64(mu.XP),
		Vials:    int64(mu.Vials),
		Lemons:   int64(mu.Lemons),
		Votes:    int64(mu.Votes),
	}
}

func convertBanInfo(bi BanInfo) models.BanInfo {
	return models.BanInfo{
		Full:    bi.Full,
		Embargo: bi.Embargo,
		Report:  bi.Report,
		Tags:    int(bi.Tags),
	}
}

func convertPreferences(prefs Preferences) *models.Preferences {
	return &models.Preferences{
		Notifications: models.NotificationPreferences{
			AucBidMe:  prefs.Notifications.AucBidMe,
			AucOutBid: prefs.Notifications.AucOutBid,
			AucNewBid: prefs.Notifications.AucNewBid,
			AucEnd:    prefs.Notifications.AucEnd,
			Announce:  prefs.Notifications.Announce,
			Daily:     prefs.Notifications.Daily,
			Vote:      prefs.Notifications.Vote,
			Completed: prefs.Notifications.Completed,
			EffectEnd: prefs.Notifications.EffectEnd,
		},
		Interactions: models.InteractionPreferences{
			CanHas:  prefs.Interactions.CanHas,
			CanDiff: prefs.Interactions.CanDiff,
			CanSell: prefs.Interactions.CanSell,
		},
		Profile: models.ProfilePreferences{
			Bio:         cleanseString(prefs.Profile.Bio),
			Title:       cleanseString(prefs.Profile.Title),
			Color:       prefs.Profile.Color,
			Card:        prefs.Profile.Card,
			FavComplete: prefs.Profile.FavComplete,
			FavClout:    prefs.Profile.FavClout,
			Image:       prefs.Profile.Image,
			Reputation:  int(prefs.Profile.Reputation),
		},
	}
}

func (m *Migrator) convertUserCard(mc MongoUserCard) *models.UserCard {
	now := time.Now()

	var cardID int64
	if mc.CardID != nil {
		cardID = int64(*mc.CardID)
	} else {
		// Decide how to handle null CardID
		// For this example, we'll skip the record
		return nil
	}

	return &models.UserCard{
		UserID:    mc.UserID,
		CardID:    cardID,
		Favorite:  mc.Fav,
		Locked:    mc.Locked,
		Amount:    int64(mc.Amount),
		Rating:    int64(mc.Rating),
		Obtained:  mc.Obtained,
		Exp:       int64(mc.Exp),
		Mark:      mc.Mark,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Extract collection IDs from []ColInfo
func extractColIDs(cols []ColInfo) []string {
	var ids []string
	for _, col := range cols {
		ids = append(ids, col.ID)
	}
	return ids
}

// Convert completed collections to new format
func convertCompletedCols(cols []ColInfo) []models.CompletedColModel {
	var result []models.CompletedColModel
	for _, col := range cols {
		result = append(result, models.CompletedColModel{
			ID:     col.ID,
			Amount: 0, // Legacy completed collections didn't track amounts
		})
	}
	return result
}

// Convert clouted collections to new format
func convertCloutedCols(cols []ColInfo) []models.CloutedColModel {
	var result []models.CloutedColModel
	for _, col := range cols {
		amount := 1 // Default amount for legacy clouted collections
		if col.Amount > 0 {
			amount = int(col.Amount)
		}
		result = append(result, models.CloutedColModel{
			ID:     col.ID,
			Amount: amount,
		})
	}
	return result
}

// Convert wishlist from []int32 to []string or []int64 as needed
func convertWishlist(wishlist []int32) []string {
	var result []string
	for _, item := range wishlist {
		result = append(result, strconv.Itoa(int(item)))
	}
	return result
}

// Convert Inventory
func convertInventory(items []interface{}) []models.InventoryItemModel {
	var result []models.InventoryItemModel
	for _, item := range items {
		switch v := item.(type) {
		case string:
			// Handle string items if needed
			fmt.Printf("Skipping string item in inventory: %s\n", v)
		case primitive.D:
			var invItem InventoryItem
			data, err := bson.Marshal(v)
			if err != nil {
				fmt.Printf("Failed to marshal inventory item: %v\n", err)
				continue
			}
			if err := bson.Unmarshal(data, &invItem); err != nil {
				fmt.Printf("Failed to unmarshal inventory item: %v\n", err)
				continue
			}
			result = append(result, models.InventoryItemModel{
				Time: invItem.Time,
				Col:  invItem.Col,
				ID:   invItem.ID,
			})
		case primitive.M:
			var invItem InventoryItem
			data, err := bson.Marshal(v)
			if err != nil {
				fmt.Printf("Failed to marshal inventory item: %v\n", err)
				continue
			}
			if err := bson.Unmarshal(data, &invItem); err != nil {
				fmt.Printf("Failed to unmarshal inventory item: %v\n", err)
				continue
			}
			result = append(result, models.InventoryItemModel{
				Time: invItem.Time,
				Col:  invItem.Col,
				ID:   invItem.ID,
			})
		default:
			fmt.Printf("Unsupported inventory item type: %T\n", v)
		}
	}
	return result
}

// cleanseString removes null bytes, invalid UTF-8 sequences, and encoding issues
// Fixed to preserve valid Unicode characters by processing UTF-8 runes instead of bytes
func cleanseString(s string) string {
	if s == "" {
		return ""
	}

	// Only apply Windows-1252 fixes to strings that are actually problematic
	// Skip this for strings that are already valid UTF-8 with Unicode characters
	if utf8.ValidString(s) {
		// For valid UTF-8 strings, only remove null bytes and control characters
		var result strings.Builder
		result.Grow(len(s))

		for _, r := range s {
			// Remove null runes and most control characters (keep tab, newline, carriage return)
			if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) {
				continue
			}
			result.WriteRune(r)
		}

		return strings.TrimSpace(result.String())
	}

	// Only for strings with encoding issues, apply Windows-1252 to UTF-8 conversion
	original := s

	// Handle common Windows-1252 to UTF-8 problematic characters
	s = strings.ReplaceAll(s, "\x90", "")  // Remove 0x90 character
	s = strings.ReplaceAll(s, "\x91", "'") // Left single quotation mark
	s = strings.ReplaceAll(s, "\x92", "'") // Right single quotation mark
	s = strings.ReplaceAll(s, "\x93", `"`) // Left double quotation mark
	s = strings.ReplaceAll(s, "\x94", `"`) // Right double quotation mark
	s = strings.ReplaceAll(s, "\x95", "•") // Bullet
	s = strings.ReplaceAll(s, "\x96", "–") // En dash
	s = strings.ReplaceAll(s, "\x97", "—") // Em dash
	s = strings.ReplaceAll(s, "\x98", "~") // Small tilde
	s = strings.ReplaceAll(s, "\x99", "™") // Trade mark
	s = strings.ReplaceAll(s, "\x9A", "š") // Latin small letter s with caron
	s = strings.ReplaceAll(s, "\x9B", "›") // Single right-pointing angle quotation mark
	s = strings.ReplaceAll(s, "\x9C", "œ") // Latin small ligature oe
	s = strings.ReplaceAll(s, "\x9D", "")  // Remove 0x9D
	s = strings.ReplaceAll(s, "\x9E", "ž") // Latin small letter z with caron
	s = strings.ReplaceAll(s, "\x9F", "Ÿ") // Latin capital letter y with diaeresis

	// Process as UTF-8 runes instead of bytes to preserve multi-byte sequences
	var result strings.Builder
	result.Grow(len(s))

	for _, r := range s {
		// Remove null runes and most control characters (keep tab, newline, carriage return)
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) {
			continue
		}
		result.WriteRune(r)
	}

	cleaned := result.String()

	// Final UTF-8 validation and cleanup
	if !utf8.ValidString(cleaned) {
		fmt.Printf("Warning: Still invalid UTF-8 after cleaning: %q -> applying fallback\n", original)
		cleaned = strings.ToValidUTF8(cleaned, "")
	}

	return strings.TrimSpace(cleaned)
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Converter functions for new BSON types following existing patterns

// Convert MongoDB collection to PostgreSQL collection
func (m *Migrator) convertCollection(mc MongoCollection) *models.Collection {
	now := time.Now()

	return &models.Collection{
		ID:         mc.ColID,
		Name:       cleanseString(mc.Name),
		Origin:     mc.Origin,
		Aliases:    mc.Aliases,
		Promo:      mc.Promo,
		Compressed: mc.Compressed,
		Fragments:  mc.Fragments,
		Tags:       mc.Tags,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// Convert MongoDB card to PostgreSQL card
func (m *Migrator) convertMongoCard(mc MongoCard) *models.Card {
	now := time.Now()

	// Convert tags string to array following existing pattern
	var tags []string
	if mc.Tags != "" {
		tags = []string{mc.Tags}
	}

	return &models.Card{
		ID:        int64(mc.CardID),
		Name:      cleanseString(mc.Name),
		Level:     int(mc.Level),
		Animated:  mc.Animated,
		ColID:     mc.ColID,
		Tags:      tags,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Convert MongoDB claim to PostgreSQL claims (array decomposition)
func (m *Migrator) convertClaims(mc MongoClaim) []*models.Claim {
	var claims []*models.Claim

	// Decompose array: create one claim record per card
	for _, cardID := range mc.Cards {
		claim := &models.Claim{
			CardID:    int64(cardID),
			UserID:    mc.User,
			ClaimedAt: mc.Date,
			Expires:   mc.Date.Add(24 * time.Hour), // Default 24h expiry
		}
		claims = append(claims, claim)
	}

	return claims
}

// Convert MongoDB auction to PostgreSQL auction and auction bids
func (m *Migrator) convertAuction(ma MongoAuction) (*models.Auction, []*models.AuctionBid) {
	now := time.Now()

	// Determine auction status
	var status models.AuctionStatus
	if ma.Cancelled {
		status = models.AuctionStatusCancelled
	} else if ma.Finished {
		status = models.AuctionStatusCompleted
	} else {
		status = models.AuctionStatusActive
	}

	// Create auction record
	auction := &models.Auction{
		AuctionID:    ma.AuctionID,
		CardID:       int64(ma.Card),
		SellerID:     ma.Author,
		StartPrice:   ma.Price,
		CurrentPrice: ma.HighBid,
		MinIncrement: 100, // Default increment
		TopBidderID:  ma.LastBidder,
		Status:       status,
		StartTime:    ma.Time,
		EndTime:      ma.Expires,
		BidCount:     len(ma.Bids),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if len(ma.Bids) > 0 {
		auction.LastBidTime = ma.Bids[len(ma.Bids)-1].Time
	}

	// Create individual bid records for relational enhancement
	var auctionBids []*models.AuctionBid
	for _, bid := range ma.Bids {
		auctionBid := &models.AuctionBid{
			BidderID:  bid.User,
			Amount:    bid.Bid,
			Timestamp: bid.Time,
			CreatedAt: now,
		}
		auctionBids = append(auctionBids, auctionBid)
	}

	return auction, auctionBids
}

// Convert MongoDB user effect to PostgreSQL user effect
func (m *Migrator) convertUserEffect(me MongoUserEffect) *models.UserEffect {
	now := time.Now()

	effect := &models.UserEffect{
		UserID:    me.UserID,
		EffectID:  me.EffectID,
		Uses:      int(me.Uses),
		Notified:  me.Notified,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Handle optional fields
	if !me.Expires.IsZero() {
		effect.ExpiresAt = &me.Expires
	}

	if !me.CooldownEnds.IsZero() {
		effect.CooldownEndsAt = &me.CooldownEnds
	}

	return effect
}

// Convert MongoDB user quest to PostgreSQL user quest
func (m *Migrator) convertUserQuest(mq MongoUserQuest) *models.UserQuest {
	now := time.Now()

	return &models.UserQuest{
		UserID:    mq.UserID,
		QuestID:   mq.QuestID,
		Type:      mq.Type,
		Completed: mq.Completed,
		CreatedAt: mq.Created,
		ExpiresAt: mq.Expiry,
		UpdatedAt: now,
	}
}

// Convert MongoDB user inventory to PostgreSQL user recipe
// MongoDB USER INVENTORIES actually represents recipes with specific cards, not simple inventory counts
func (m *Migrator) convertUserInventory(mi MongoUserInventory) *models.UserRecipe {
	now := time.Now()

	// Convert int32 card IDs to int64
	var cardIDs []int64
	for _, cardID := range mi.Cards {
		cardIDs = append(cardIDs, int64(cardID))
	}

	// MongoDB USER INVENTORIES should map to UserRecipe, not UserInventory
	// This preserves the specific card information
	recipe := &models.UserRecipe{
		UserID:    mi.UserID,
		ItemID:    mi.ItemID,
		CardIDs:   cardIDs,
		CreatedAt: mi.Acquired, // Use original acquired time
		UpdatedAt: now,
	}

	return recipe
}

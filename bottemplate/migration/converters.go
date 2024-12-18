// converters.go
package migration

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"

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
		CompletedCols:   extractColIDs(mu.CompletedCols),
		CloutedCols:     extractColIDs(mu.CloutedCols),
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

func cleanseString(input string) string {
	output := ""
	for _, char := range input {
		if char < 32 || char > 126 {
			slog.Warn("Removed invalid character from string", "char", char, "input", input)
			continue
		}
		output += string(char)
	}
	return output
}

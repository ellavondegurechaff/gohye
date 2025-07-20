package effects

import "github.com/disgoorg/bot-template/bottemplate/database/models"

// EffectItemData represents the static data for effect items available in the shop
type EffectItemData struct {
	ID          string
	Name        string
	Description string
	Type        string
	Price       int64
	Currency    string
	Recipe      []int64
	Duration    int
	Passive     bool
	Cooldown    int
	TierData    *models.EffectTierData // Tier progression data (nil for non-tiered effects)
}

// StaticEffectItems contains all effect items available for purchase
// This replaces the need for a database seeder or JSON file
var StaticEffectItems = []EffectItemData{
	// Passive Effects
	{
		ID:          "tohrugift",
		Name:        "Gift From Tohru",
		Description: "Increase chances of getting a 3-star card every first claim per day",
		Type:        models.EffectTypeRecipe,
		Price:       4500,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 2, 2},
		Duration:    210,
		Passive:     true,
		Cooldown:    0,
	},
	{
		ID:          "cakeday",
		Name:        "Cake Day",
		Description: "Get extra flakes per daily for every claim you did",
		Type:        models.EffectTypeRecipe,
		Price:       20000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 2, 2, 2, 2},
		Duration:    210,
		Passive:     true,
		Cooldown:    0, TierData: &models.EffectTierData{
			Values:     []int{10, 25, 45, 70, 100}, // Flakes per claim
			Thresholds: []int{100, 300, 700, 1500}, // Claims needed for next tier
		},
	},
	{Description: "Get extra vials per liquify",
		Type:     models.EffectTypeRecipe,
		Price:    22500,
		Currency: models.CurrencyTomato,
		Recipe:   []int64{2, 2, 3, 3},
		Duration: 210,
		Passive:  true, TierData: &models.EffectTierData{
			Values:     []int{5, 10, 20, 40, 70}, // Extra vials per liquify
			Thresholds: []int{30, 80, 180, 350},  // Liquefies needed for next tier
		},
	},
	{Description: "Gain cashback from winning auctions",
		Type:     models.EffectTypeRecipe,
		Price:    35000,
		Currency: models.CurrencyTomato,
		Recipe:   []int64{1, 2, 2, 2, 2},
		Duration: 210,
		Passive:  true, TierData: &models.EffectTierData{
			Values:     []int{2, 4, 6, 8, 10},               // Cashback percentage
			Thresholds: []int{20000, 60000, 150000, 350000}, // Flakes spent on wins
		},
	},
	{Description: "Reduce forge and ascend cost",
		Type:     models.EffectTypeRecipe,
		Price:    20000,
		Currency: models.CurrencyTomato,
		Recipe:   []int64{2, 2, 2, 3},
		Duration: 210,
		Passive:  true, TierData: &models.EffectTierData{
			Values:     []int{20, 30, 40, 50, 60}, // Discount percentage
			Thresholds: []int{10, 30, 70, 150},    // Forges + ascends
		},
	},
	{Description: "Reduce daily cooldown",
		Type:     models.EffectTypeRecipe,
		Price:    30000,
		Currency: models.CurrencyTomato,
		Recipe:   []int64{3, 3, 3},
		Duration: 210,
		Passive:  true, TierData: &models.EffectTierData{
			Values:     []int{1170, 1140, 1110, 1080, 1020}, // Minutes (19.5h, 19h, 18.5h, 18h, 17h)
			Thresholds: []int{10, 25, 50, 100},              // Dailies
		},
	},
	{
		ID:          "spellcard",
		Name:        "Impossible Spell Card",
		Description: "Usable effects have 40% less cooldown",
		Type:        models.EffectTypeRecipe,
		Price:       5000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 2, 2, 3, 3, 3},
		Duration:    210,
		Passive:     true,
		Cooldown:    0,
	},
	{
		ID:          "walpurgisnight",
		Name:        "Walpurgis Night",
		Description: "Draw few times per daily, maximum of 3 star per daily",
		Type:        models.EffectTypeRecipe,
		Price:       15000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 3, 3, 3, 3},
		Duration:    50,
		Passive:     true,
		Cooldown:    0,
	}, {
		ID:          "lambhyejoo",
		Name:        "Lamb of Hyejoo",
		Description: "Gain extra flakes from selling cards on auction",
		Type:        models.EffectTypeRecipe,
		Price:       25000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{1, 2, 2, 2, 2},
		Duration:    210,
		Passive:     true,
		Cooldown:    0,

		TierData: &models.EffectTierData{
			Values:     []int{2, 4, 6, 8, 10},               // Sale bonus percentage
			Thresholds: []int{20000, 60000, 150000, 350000}, // Flakes earned from sales
		},
	},
	{
		ID:          "youthyouth",
		Name:        "Youth Youth By Young",
		Description: "Gain extra rewards from work",
		Type:        models.EffectTypeRecipe,
		Price:       15000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 2, 2, 2, 2},
		Duration:    210,
		Passive:     true,
		Cooldown:    0,

		TierData: &models.EffectTierData{
			Values:     []int{10, 20, 30, 40, 50}, // Work bonus percentage
			Thresholds: []int{50, 150, 350, 700},  // Works needed
		},
	},
	{
		ID:          "kisslater",
		Name:        "Kiss Later",
		Description: "Gain extra XP from levelup",
		Type:        models.EffectTypeRecipe,
		Price:       15000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 2, 2, 2, 2},
		Duration:    210,
		Passive:     true,
		Cooldown:    0,

		TierData: &models.EffectTierData{
			Values:     []int{5, 10, 15, 20, 30}, // XP bonus percentage
			Thresholds: []int{30, 80, 180, 400},  // Levelups needed
		},
	},

	// Active Effects
	{
		ID:          "enayano",
		Name:        "Enlightened Ayano",
		Description: "Completes tier 1 quest when used",
		Type:        models.EffectTypeRecipe,
		Price:       5500,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{1, 1, 2, 2},
		Duration:    14,
		Passive:     false,
		Cooldown:    20,
	},
	{
		ID:          "pbocchi",
		Name:        "Powerful Bocchi",
		Description: "Generates tier 1 quest when used",
		Type:        models.EffectTypeRecipe,
		Price:       7000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 2, 3, 3},
		Duration:    14,
		Passive:     false,
		Cooldown:    32,
	},
	{
		ID:          "spaceunity",
		Name:        "The Space Unity",
		Description: "Gives random unique card from non-promo collection",
		Type:        models.EffectTypeRecipe,
		Price:       15000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 2, 3, 3},
		Duration:    8,
		Passive:     false,
		Cooldown:    40,
	},
	{
		ID:          "judgeday",
		Name:        "The Judgment Day",
		Description: "Grants effect of almost any usable card",
		Type:        models.EffectTypeRecipe,
		Price:       12000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 3, 3, 3},
		Duration:    14,
		Passive:     false,
		Cooldown:    48,
	},
	{
		ID:          "claimrecall",
		Name:        "Claim Recall",
		Description: "Claim cost gets recalled by 4 claims, as if they never happened",
		Type:        models.EffectTypeRecipe,
		Price:       10000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{1, 1, 1, 2, 2, 2},
		Duration:    20,
		Passive:     false,
		Cooldown:    15,
	},
}

// GetEffectItemByID returns an effect item by its ID
func GetEffectItemByID(id string) *EffectItemData {
	for i := range StaticEffectItems {
		if StaticEffectItems[i].ID == id {
			return &StaticEffectItems[i]
		}
	}
	return nil
}

// GetPassiveEffectItems returns all passive effect items
func GetPassiveEffectItems() []EffectItemData {
	var items []EffectItemData
	for _, item := range StaticEffectItems {
		if item.Passive {
			items = append(items, item)
		}
	}
	return items
}

// GetActiveEffectItems returns all active effect items
func GetActiveEffectItems() []EffectItemData {
	var items []EffectItemData
	for _, item := range StaticEffectItems {
		if !item.Passive {
			items = append(items, item)
		}
	}
	return items
}

// ToEffectItem converts EffectItemData to models.EffectItem
func (e *EffectItemData) ToEffectItem() *models.EffectItem {
	effectType := e.Type
	// Convert recipe type to appropriate active/passive type for shop display
	if e.Type == models.EffectTypeRecipe {
		if e.Passive {
			effectType = models.EffectTypePassive
		} else {
			effectType = models.EffectTypeActive
		}
	}

	return &models.EffectItem{
		ID:          e.ID,
		Name:        e.Name,
		Description: e.Description,
		Type:        effectType,
		Price:       e.Price,
		Currency:    e.Currency,
		Duration:    e.Duration,
		Level:       0,
		Recipe:      e.Recipe,
		Cooldown:    e.Cooldown,
		Passive:     e.Passive,
	}
}

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
		ID:          "cakeday",
		Name:        "Cake Day",
		Description: "Get extra flakes per daily for every claim you did",
		Type:        models.EffectTypeRecipe,
		Price:       15000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 2, 2, 2, 2},
		Duration:    210,
		Passive:     true,
		Cooldown:    0,
		TierData: &models.EffectTierData{
			Values:     []int{10, 25, 45, 70, 100},  // Flakes per claim
			Thresholds: []int{250, 500, 1000, 1500}, // Claims needed for next tier
		},
	},
	{
		ID:          "holygrail",
		Name:        "Holy Grail",
		Description: "Get extra vials per liquify",
		Type:        models.EffectTypeRecipe,
		Price:       15000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 2, 3, 3},
		Duration:    210,
		Passive:     true,
		Cooldown:    0,
		TierData: &models.EffectTierData{
			Values:     []int{5, 10, 20, 40, 70}, // Extra vials per liquify
			Thresholds: []int{70, 150, 250, 500}, // Liquefies needed for next tier
		},
	},
	{
		ID:          "wolfofhyejoo",
		Name:        "Wolf of Hyejoo",
		Description: "Gain cashback from winning auctions",
		Type:        models.EffectTypeRecipe,
		Price:       25000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{1, 2, 2, 2, 2},
		Duration:    210,
		Passive:     true,
		Cooldown:    0,
		TierData: &models.EffectTierData{
			Values:     []int{2, 4, 6, 8, 10},               // Cashback percentage
			Thresholds: []int{30000, 75000, 150000, 350000}, // Flakes spent on wins
		},
	},
	{
		ID:          "cherrybloss",
		Name:        "Cherry Blossom",
		Description: "Reduce forge and ascend cost",
		Type:        models.EffectTypeRecipe,
		Price:       15000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 2, 2, 3},
		Duration:    210,
		Passive:     true,
		Cooldown:    0,
		TierData: &models.EffectTierData{
			Values:     []int{20, 30, 40, 50, 50}, // Discount percentage
			Thresholds: []int{20, 50, 5, 10},      // Forge/ascend milestones from effects.txt
		},
	},
	{
		ID:          "rulerjeanne",
		Name:        "Ruler Jeanne",
		Description: "Reduce daily cooldown",
		Type:        models.EffectTypeRecipe,
		Price:       20000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{3, 3, 3},
		Duration:    210,
		Passive:     true,
		Cooldown:    0,
		TierData: &models.EffectTierData{
			Values:     []int{1170, 1140, 1110, 1080, 1020}, // Minutes (19.5h, 19h, 18.5h, 18h, 17h)
			Thresholds: []int{20, 40, 65, 85},               // Dailies
		},
	},
	{
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
			Thresholds: []int{30000, 75000, 150000, 350000}, // Flakes earned from sales
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
			Thresholds: []int{150, 300, 500, 750}, // Works needed
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
			Values:     []int{5, 10, 15, 20, 30},       // XP bonus percentage
			Thresholds: []int{150, 300, 450, 600, 850}, // Levelups needed
		},
	},

	// Items
	{
		ID:          "spaceunity",
		Name:        "Space Unity",
		Description: "Gives a random unique card from a non-promo collection of choice, excluding 4 and 5 star cards",
		Type:        models.EffectTypeRecipe,
		Price:       12000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 2, 3, 3},
		Duration:    8,
		Passive:     false,
		Cooldown:    40,
	},
	{
		ID:          "judgeday",
		Name:        "Judgement Day",
		Description: "Can be used as any other item",
		Type:        models.EffectTypeRecipe,
		Price:       16000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 2, 3, 3, 3},
		Duration:    14,
		Passive:     false,
		Cooldown:    48,
	},
	{
		ID:          "walpurgisnight",
		Name:        "Walpurgis Night",
		Description: "Grants an extra draw",
		Type:        models.EffectTypeRecipe,
		Price:       10000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{1, 2, 3, 3},
		Duration:    20,
		Passive:     false,
		Cooldown:    24,
	},
	{
		ID:          "claimrecall",
		Name:        "Claim Recall",
		Description: "Claim cost gets reset by 4 claims",
		Type:        models.EffectTypeRecipe,
		Price:       10000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{1, 1, 1, 2, 2},
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

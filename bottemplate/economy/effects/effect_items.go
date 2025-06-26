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
	Animated    bool
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
		Animated:    false,
	},
	{
		ID:          "cakeday",
		Name:        "Cake Day",
		Description: "Get +100 snowflakes in your daily for every claim you did",
		Type:        models.EffectTypeRecipe,
		Price:       20000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 2, 2, 2, 2},
		Duration:    210,
		Passive:     true,
		Cooldown:    0,
		Animated:    true,
	},
	{
		ID:          "holygrail",
		Name:        "The Holy Grail",
		Description: "Get +25% of vials when liquifying 1 and 2-star cards",
		Type:        models.EffectTypeRecipe,
		Price:       22500,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 2, 3, 3},
		Duration:    210,
		Passive:     true,
		Cooldown:    0,
		Animated:    true,
	},
	{
		ID:          "skyfriend",
		Name:        "Skies Of Friendship",
		Description: "Get 10% snowflakes back from wins on auction",
		Type:        models.EffectTypeRecipe,
		Price:       35000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{1, 2, 2, 2, 2},
		Duration:    210,
		Passive:     true,
		Cooldown:    0,
		Animated:    false,
	},
	{
		ID:          "cherrybloss",
		Name:        "Cherry Blossoms",
		Description: "Any card forge is 50% cheaper",
		Type:        models.EffectTypeRecipe,
		Price:       20000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{2, 2, 2, 3},
		Duration:    210,
		Passive:     true,
		Cooldown:    0,
		Animated:    false,
	},
	{
		ID:          "rulerjeanne",
		Name:        "The Ruler Jeanne",
		Description: "Get `/daily` every 17 hours instead of 20",
		Type:        models.EffectTypeRecipe,
		Price:       30000,
		Currency:    models.CurrencyTomato,
		Recipe:      []int64{3, 3, 3},
		Duration:    210,
		Passive:     true,
		Cooldown:    0,
		Animated:    false,
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
		Animated:    false,
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
		Animated:    false,
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
		Animated:    false,
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
		Animated:    false,
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
		Animated:    false,
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
		Animated:    false,
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
		Animated:    false,
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
	if e.Passive {
		effectType = models.EffectTypePassive
	} else {
		effectType = models.EffectTypeActive
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
		Animated:    e.Animated,
	}
}
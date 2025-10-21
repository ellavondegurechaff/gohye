package economy

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Work = discord.SlashCommandCreate{
	Name:        "work",
	Description: "💼 Work in the K-pop industry to earn rewards",
}

type WorkHandler struct {
	bot *bottemplate.Bot
}

func NewWorkHandler(b *bottemplate.Bot) *WorkHandler {
	return &WorkHandler{bot: b}
}

const workCooldown = config.WorkMinCooldown

type JobRarity int

const (
	JobRarity1Star JobRarity = iota + 1
	JobRarity2Star
	JobRarity3Star
	JobRarity4Star
	JobRarity5Star
)

type JobScenario struct {
	Title       string
	Description string
	Question    string
	Options     []string
	CorrectIdx  int
	Rarity      JobRarity
}

type WorkRewards struct {
	Flakes    int64
	Vials     int64
	XP        int64
	ItemDrops []string // Item IDs that were dropped
}

// Industry scenario types
type ScenarioType string

const (
	ScenarioMusicProduction ScenarioType = "music_production"
	ScenarioVarietyShow     ScenarioType = "variety_show"
	ScenarioConcertPlanning ScenarioType = "concert_planning"
	ScenarioPhotoshoot      ScenarioType = "photoshoot"
)

// Enhanced job scenario with card integration
type EnhancedJobScenario struct {
	JobScenario
	Type             ScenarioType
	RequiredTags     []string // Tags that give bonuses
	CollectionBonus  string   // Specific collection that gives bonus
	MinCardsForBonus int      // Minimum cards needed for bonus
}

// Card bonus calculation result
type CardBonus struct {
	HasTagBonus          bool
	HasCollectionBonus   bool
	TagMultiplier        float64
	CollectionMultiplier float64
	CombinedMultiplier   float64
	RelevantCards        []*models.Card // Cards from the specific collection
	CollectionName       string
	TagMatchCount        int // Count of cards matching required tags
	CollectionCardCount  int // Count of cards from the specific collection
}

func (h *WorkHandler) HandleWork(e *handler.CommandEvent) error {
    ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
    defer cancel()

    // Defer immediately to avoid 3s timeout and 10062
    if err := e.DeferCreateMessage(false); err != nil {
        return err
    }

	// Check cooldown - DISABLED FOR TESTING
	// user, err := h.bot.UserRepository.GetByDiscordID(ctx, e.User().ID.String())
	// if err != nil {
	// 	return utils.EH.CreateErrorEmbed(e, "Failed to fetch user data")
	// }

	// Cooldown check disabled for testing
	// if time.Since(user.LastWork) < workCooldown {
	// 	remaining := time.Until(user.LastWork.Add(workCooldown)).Round(time.Second)
	// 	return utils.EH.CreateErrorEmbed(e, fmt.Sprintf("You need to rest for %s before working again!", remaining))
	// }

	// Generate job scenario with collection bonus resolved
	scenario, enhancedScenario := h.generateJobScenarioWithCollection(ctx)

	// Create embed with collection info
	embed := h.createJobScenarioEmbedWithCollection(scenario, enhancedScenario)
	components := h.createScenarioComponentsWithCollection(scenario, enhancedScenario, e.User().ID.String())

    _, err := e.UpdateInteractionResponse(discord.MessageUpdate{
        Embeds:     &[]discord.Embed{embed},
        Components: &components,
    })
    return err
}

func (h *WorkHandler) HandleComponent(e *handler.ComponentEvent) error {
    // Acknowledge immediately to prevent 3s timeout (10062)
    _ = e.DeferUpdateMessage()

    parts := strings.Split(e.Data.CustomID(), "/")
    if len(parts) < 2 {
        _, err := e.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr("❌ Invalid interaction")})
        return err
    }

	action := parts[1]
	switch action {
	case "answer":
		// Check for old format (backward compatibility)
		if len(parts) == 5 {
			// Old format without collection
			var correctIdx, chosenIdx, rarity int
			fmt.Sscanf(parts[2], "%d", &correctIdx)
			fmt.Sscanf(parts[3], "%d", &chosenIdx)
			fmt.Sscanf(parts[4], "%d", &rarity)
			return h.HandleWorkAnswer(e, correctIdx, chosenIdx, JobRarity(rarity), "")
		} else if len(parts) == 6 {
			// Old format with collection but no user validation
			var correctIdx, chosenIdx, rarity int
			fmt.Sscanf(parts[2], "%d", &correctIdx)
			fmt.Sscanf(parts[3], "%d", &chosenIdx)
			fmt.Sscanf(parts[4], "%d", &rarity)
			collectionID := parts[5]
			if collectionID == "none" {
				collectionID = ""
			}
            // After deferring, don't return REST call result; return nil
            _ = h.HandleWorkAnswer(e, correctIdx, chosenIdx, JobRarity(rarity), collectionID)
            return nil
		} else if len(parts) >= 7 {
			// New format with user validation
			var correctIdx, chosenIdx, rarity int
			fmt.Sscanf(parts[2], "%d", &correctIdx)
			fmt.Sscanf(parts[3], "%d", &chosenIdx)
			fmt.Sscanf(parts[4], "%d", &rarity)
			collectionID := parts[5]
			originalUserID := parts[6]

			// Validate that only the command author can click
            if e.User().ID.String() != originalUserID {
                // After deferring, send ephemeral follow-up instead of a second interaction response
                _, _ = e.CreateFollowupMessage(discord.MessageCreate{
                    Content: "Only the person who used the /work command can answer this question.",
                    Flags:   discord.MessageFlagEphemeral,
                })
                return nil
            }

			if collectionID == "none" {
				collectionID = ""
			}
			return h.HandleWorkAnswer(e, correctIdx, chosenIdx, JobRarity(rarity), collectionID)
		} else {
            _, err := e.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr("❌ Invalid answer format")})
            return err
        }
    default:
        _, err := e.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr("❌ Invalid action")})
        return err
    }
}

func (h *WorkHandler) generateJobScenarioWithCollection(ctx context.Context) (JobScenario, EnhancedJobScenario) {
	// Determine rarity (weighted distribution)
	rarityRoll := rand.Intn(100)
	var rarity JobRarity
	switch {
	case rarityRoll < 40: // 40% chance
		rarity = JobRarity1Star
	case rarityRoll < 70: // 30% chance
		rarity = JobRarity2Star
	case rarityRoll < 85: // 15% chance
		rarity = JobRarity3Star
	case rarityRoll < 95: // 10% chance
		rarity = JobRarity4Star
	default: // 5% chance
		rarity = JobRarity5Star
	}

	// Use enhanced scenarios for card integration
	enhancedScenarios := getEnhancedJobScenarios(rarity)
	enhanced := enhancedScenarios[rand.Intn(len(enhancedScenarios))]

	// If scenario has "random" collection bonus, pick a random collection
	if enhanced.CollectionBonus == "random" {
		collections, err := h.bot.CollectionRepository.GetAll(ctx)
		if err == nil && len(collections) > 0 {
			// Filter out special collections (fragments, albums, etc)
			var regularCollections []*models.Collection
			for _, col := range collections {
				if !col.Fragments && !col.Promo {
					regularCollections = append(regularCollections, col)
				}
			}
			if len(regularCollections) > 0 {
				randomCol := regularCollections[rand.Intn(len(regularCollections))]
				enhanced.CollectionBonus = randomCol.ID
			}
		}
	}

	return enhanced.JobScenario, enhanced
}

// Generate enhanced scenarios based on rarity
func getEnhancedJobScenarios(rarity JobRarity) []EnhancedJobScenario {
	switch rarity {
	case JobRarity1Star:
		return []EnhancedJobScenario{
			{
				JobScenario: JobScenario{
					Title:       "🎵 Trainee Practice Session",
					Description: "You're helping trainees prepare for their monthly evaluation.",
					Question:    "What should they focus on?",
					Options:     []string{"Vocal stability", "Dance synchronization", "Stage presence", "Everything equally"},
					CorrectIdx:  1,
					Rarity:      JobRarity1Star,
				},
				Type:             ScenarioMusicProduction,
				RequiredTags:     []string{"girlgroups", "boygroups"},
				CollectionBonus:  "random",
				MinCardsForBonus: 3,
			},
			{
				JobScenario: JobScenario{
					Title:       "📸 SNS Content Creation",
					Description: "An idol needs help choosing photos for their Instagram.",
					Question:    "Which concept works best for engagement?",
					Options:     []string{"Casual selfies", "Professional shots", "Behind-the-scenes", "Food photos"},
					CorrectIdx:  2,
					Rarity:      JobRarity1Star,
				},
				Type:             ScenarioPhotoshoot,
				RequiredTags:     []string{"girlgroups", "boygroups"},
				CollectionBonus:  "random",
				MinCardsForBonus: 2,
			},
		}
	case JobRarity2Star:
		return []EnhancedJobScenario{
			{
				JobScenario: JobScenario{
					Title:       "🎬 Music Show Recording",
					Description: "You're coordinating a group's music show performance.",
					Question:    "The camera director asks for the killing part. When should it be?",
					Options:     []string{"Opening verse", "First chorus", "Dance break", "Final chorus"},
					CorrectIdx:  2,
					Rarity:      JobRarity2Star,
				},
				Type:             ScenarioConcertPlanning,
				RequiredTags:     []string{"girlgroups", "boygroups"},
				CollectionBonus:  "random", // Will be randomized
				MinCardsForBonus: 5,
			},
			{
				JobScenario: JobScenario{
					Title:       "🎪 Variety Show Guest",
					Description: "Your idol is appearing on Running Man.",
					Question:    "What game would showcase them best?",
					Options:     []string{"Name tag elimination", "Quiz games", "Dance battles", "Cooking challenge"},
					CorrectIdx:  0,
					Rarity:      JobRarity2Star,
				},
				Type:             ScenarioVarietyShow,
				RequiredTags:     []string{"girlgroups", "boygroups"},
				CollectionBonus:  "random",
				MinCardsForBonus: 4,
			},
		}
	case JobRarity3Star:
		return []EnhancedJobScenario{
			{
				JobScenario: JobScenario{
					Title:       "🎹 Album Production Meeting",
					Description: "Planning the concept for a major group's comeback.",
					Question:    "The group wants to try something new. What direction?",
					Options:     []string{"Retro synthwave", "Dark & mysterious", "Bright summer", "R&B influenced"},
					CorrectIdx:  3,
					Rarity:      JobRarity3Star,
				},
				Type:             ScenarioMusicProduction,
				CollectionBonus:  "random",
				MinCardsForBonus: 6,
			},
			{
				JobScenario: JobScenario{
					Title:       "📸 Magazine Cover Shoot",
					Description: "Directing a high-fashion photoshoot for Vogue Korea.",
					Question:    "The theme is 'Future Nostalgia'. What's the key element?",
					Options:     []string{"Neon lighting", "Film grain effects", "Minimalist sets", "Mixed eras styling"},
					CorrectIdx:  3,
					Rarity:      JobRarity3Star,
				},
				Type:             ScenarioPhotoshoot,
				RequiredTags:     []string{"girlgroups"},
				CollectionBonus:  "random",
				MinCardsForBonus: 7,
			},
		}
	case JobRarity4Star:
		return []EnhancedJobScenario{
			{
				JobScenario: JobScenario{
					Title:       "🎪 World Tour Planning",
					Description: "Organizing a 20-city world tour for a top group.",
					Question:    "Fans are requesting an encore stage concept. What works globally?",
					Options:     []string{"Local language covers", "Fan song dedications", "Acoustic versions", "Dance unit stages"},
					CorrectIdx:  1,
					Rarity:      JobRarity4Star,
				},
				Type:             ScenarioConcertPlanning,
				CollectionBonus:  "random",
				MinCardsForBonus: 8,
			},
			{
				JobScenario: JobScenario{
					Title:       "🎬 Drama OST Production",
					Description: "Producing an OST for a major K-drama.",
					Question:    "The scene needs emotional impact. What instrumentation?",
					Options:     []string{"Full orchestra", "Piano and strings", "Acoustic guitar", "Electronic ambient"},
					CorrectIdx:  1,
					Rarity:      JobRarity4Star,
				},
				Type:             ScenarioMusicProduction,
				RequiredTags:     []string{"girlgroups", "boygroups"},
				MinCardsForBonus: 10,
			},
		}
	case JobRarity5Star:
		return []EnhancedJobScenario{
			{
				JobScenario: JobScenario{
					Title:       "🌟 Entertainment Company CEO",
					Description: "Making a crucial decision for your company's future.",
					Question:    "A legendary producer wants to work with your rookie group. The catch?",
					Options:     []string{"Expensive but guaranteed hit", "Creative control issues", "Delays other projects", "All worth it"},
					CorrectIdx:  3,
					Rarity:      JobRarity5Star,
				},
				Type:             ScenarioMusicProduction,
				RequiredTags:     []string{"girlgroups", "boygroups"},
				MinCardsForBonus: 15,
			},
			{
				JobScenario: JobScenario{
					Title:       "🏟️ Stadium Concert Director",
					Description: "Directing a 70,000 capacity stadium show.",
					Question:    "Technical rehearsal reveals a stage design flaw. Your call?",
					Options:     []string{"Cancel the effect", "Improvise a solution", "Delay the show", "Trust the team's fix"},
					CorrectIdx:  3,
					Rarity:      JobRarity5Star,
				},
				Type:             ScenarioConcertPlanning,
				CollectionBonus:  "random",
				MinCardsForBonus: 12,
			},
		}
	default:
		return []EnhancedJobScenario{getEnhancedJobScenarios(JobRarity1Star)[0]}
	}
}

// Calculate card bonus for a scenario
func (h *WorkHandler) calculateCardBonusWithCollection(ctx context.Context, userID string, scenario EnhancedJobScenario, userCards []*models.UserCard, allCards []*models.Card) CardBonus {
	bonus := CardBonus{
		HasTagBonus:          false,
		HasCollectionBonus:   false,
		TagMultiplier:        1.0,
		CollectionMultiplier: 1.0,
		CombinedMultiplier:   1.0,
		RelevantCards:        make([]*models.Card, 0),
		CollectionName:       "",
		TagMatchCount:        0,
		CollectionCardCount:  0,
	}

	// Get collection name if scenario has collection bonus
	if scenario.CollectionBonus != "" && scenario.CollectionBonus != "none" {
		collection, err := h.bot.CollectionRepository.GetByID(ctx, scenario.CollectionBonus)
		if err == nil && collection != nil {
			bonus.CollectionName = collection.Name
		}
	}

	// Create a map of card ID to card for quick lookup
	cardMap := make(map[int64]*models.Card)
	for _, card := range allCards {
		cardMap[card.ID] = card
	}

	// Count relevant cards
	tagCardCount := 0
	highestLevelTag := 0
	hasAnimatedTag := false

	highestLevelCollection := 0
	hasAnimatedCollection := false

	for _, userCard := range userCards {
		if userCard.Amount <= 0 {
			continue
		}

		card, exists := cardMap[userCard.CardID]
		if !exists {
			continue
		}

		// Check if card matches required tags
		cardMatchesTags := false
		for _, tag := range scenario.RequiredTags {
			for _, cardTag := range card.Tags {
				if strings.Contains(strings.ToLower(cardTag), strings.ToLower(tag)) {
					cardMatchesTags = true
					break
				}
			}
		}

		// Track tag-based cards
		if cardMatchesTags {
			tagCardCount++
			bonus.TagMatchCount++
			if userCard.Level > highestLevelTag {
				highestLevelTag = userCard.Level
			}
			if card.Animated {
				hasAnimatedTag = true
			}
		}

		// Track collection-based cards
		if scenario.CollectionBonus != "" && card.ColID == scenario.CollectionBonus {
			bonus.RelevantCards = append(bonus.RelevantCards, card)
			bonus.CollectionCardCount++
			if userCard.Level > highestLevelCollection {
				highestLevelCollection = userCard.Level
			}
			if card.Animated {
				hasAnimatedCollection = true
			}
		}
	}

	// Calculate tag bonus (max 1.5x)
	if tagCardCount >= 3 {
		bonus.HasTagBonus = true
		// Base: 5% per card, capped at 50% (10 cards)
		cardBonus := math.Min(float64(tagCardCount)*0.02, 0.2)
		// Level bonus: 5% per level above 1
		levelBonus := float64(highestLevelTag-1) * 0.05
		// Animated bonus: 10%
		animatedBonus := 0.0
		if hasAnimatedTag {
			animatedBonus = 0.1
		}

		bonus.TagMultiplier = 1.0 + cardBonus + levelBonus + animatedBonus
		// Cap tag multiplier at 1.5x
		if bonus.TagMultiplier > 1.5 {
			bonus.TagMultiplier = 1.5
		}
	}

	// Calculate collection bonus (max 1.5x)
	if bonus.CollectionCardCount >= 3 {
		bonus.HasCollectionBonus = true
		// Base: 10% per card, capped at 50% (5 cards)
		cardBonus := math.Min(float64(bonus.CollectionCardCount)*0.1, 0.5)
		// Level bonus: 5% per level above 1
		levelBonus := float64(highestLevelCollection-1) * 0.05
		// Animated bonus: 10%
		animatedBonus := 0.0
		if hasAnimatedCollection {
			animatedBonus = 0.1
		}

		bonus.CollectionMultiplier = 1.0 + cardBonus + levelBonus + animatedBonus
		// Cap collection multiplier at 1.5x
		if bonus.CollectionMultiplier > 1.5 {
			bonus.CollectionMultiplier = 1.5
		}
	}

	// Calculate combined multiplier
	// Multiply the bonuses together, then cap at 3.0
	bonus.CombinedMultiplier = bonus.TagMultiplier * bonus.CollectionMultiplier
	if bonus.CombinedMultiplier > 3.0 {
		bonus.CombinedMultiplier = 3.0
	}

	return bonus
}

func (h *WorkHandler) createJobScenarioEmbedWithCollection(scenario JobScenario, enhanced EnhancedJobScenario) discord.Embed {
	// Rarity display
	stars := strings.Repeat("⭐", int(scenario.Rarity))

	var description strings.Builder
	description.WriteString(fmt.Sprintf("**Rarity:** %s\n", stars))

	// Show collection bonus if applicable
	if enhanced.CollectionBonus != "" && enhanced.CollectionBonus != "random" {
		collection, err := h.bot.CollectionRepository.GetByID(context.Background(), enhanced.CollectionBonus)
		if err == nil && collection != nil {
			description.WriteString(fmt.Sprintf("**Collection Bonus:** %s (need %d+ cards)\n", collection.Name, enhanced.MinCardsForBonus))
		}
	}

	description.WriteString(fmt.Sprintf("\n*%s*\n\n", scenario.Description))
	description.WriteString(fmt.Sprintf("**%s**", scenario.Question))

	color := getRarityColor(scenario.Rarity)

	footer := "Choose wisely! You have 30 seconds."
	if enhanced.CollectionBonus != "" {
		footer += " | Bonus cards boost rewards!"
	}

	return discord.NewEmbedBuilder().
		SetTitle(scenario.Title).
		SetDescription(description.String()).
		SetColor(color).
		SetFooter(footer, "").
		Build()
}

func getRarityColor(rarity JobRarity) int {
	switch rarity {
	case JobRarity1Star:
		return 0x95a5a6 // Gray
	case JobRarity2Star:
		return 0x2ecc71 // Green
	case JobRarity3Star:
		return 0x3498db // Blue
	case JobRarity4Star:
		return 0x9b59b6 // Purple
	case JobRarity5Star:
		return 0xf39c12 // Orange
	default:
		return config.BackgroundColor
	}
}

func (h *WorkHandler) createScenarioComponentsWithCollection(scenario JobScenario, enhanced EnhancedJobScenario, userID string) []discord.ContainerComponent {
	// Store collection ID in the custom ID
	collectionID := enhanced.CollectionBonus
	if collectionID == "" {
		collectionID = "none"
	}

	buttons := make([]discord.InteractiveComponent, len(scenario.Options))

	// Create two rows if we have 4 options
	if len(scenario.Options) == 4 {
		row1 := []discord.InteractiveComponent{
			discord.NewSecondaryButton(scenario.Options[0], fmt.Sprintf("work/answer/%d/0/%d/%s/%s", scenario.CorrectIdx, scenario.Rarity, collectionID, userID)),
			discord.NewSecondaryButton(scenario.Options[1], fmt.Sprintf("work/answer/%d/1/%d/%s/%s", scenario.CorrectIdx, scenario.Rarity, collectionID, userID)),
		}
		row2 := []discord.InteractiveComponent{
			discord.NewSecondaryButton(scenario.Options[2], fmt.Sprintf("work/answer/%d/2/%d/%s/%s", scenario.CorrectIdx, scenario.Rarity, collectionID, userID)),
			discord.NewSecondaryButton(scenario.Options[3], fmt.Sprintf("work/answer/%d/3/%d/%s/%s", scenario.CorrectIdx, scenario.Rarity, collectionID, userID)),
		}

		return []discord.ContainerComponent{
			discord.NewActionRow(row1...),
			discord.NewActionRow(row2...),
		}
	}

	// Single row for fewer options
	for i, option := range scenario.Options {
		buttons[i] = discord.NewSecondaryButton(option, fmt.Sprintf("work/answer/%d/%d/%d/%s/%s", scenario.CorrectIdx, i, scenario.Rarity, collectionID, userID))
	}
	return []discord.ContainerComponent{discord.NewActionRow(buttons...)}
}

func (h *WorkHandler) HandleWorkAnswer(e *handler.ComponentEvent, correctIdx, chosenIdx int, rarity JobRarity, collectionID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.WorkHandlerTimeout)
	defer cancel()

	success := correctIdx == chosenIdx

	// Get enhanced scenario for this rarity to check card bonuses
	enhancedScenarios := getEnhancedJobScenarios(rarity)
	var scenario EnhancedJobScenario
	if len(enhancedScenarios) > 0 {
		scenario = enhancedScenarios[0] // Use first scenario of this rarity
	}

	// Use the collection ID passed from the component
	if collectionID != "" {
		scenario.CollectionBonus = collectionID
	}

	// Fetch user's cards for bonus calculation
	userID := e.User().ID.String()
	userCards, err := h.bot.UserCardRepository.GetAllByUserID(ctx, userID)
	if err != nil {
		userCards = []*models.UserCard{} // Continue without bonuses if error
	}

	// Fetch all cards for reference
	allCards, err := h.bot.CardRepository.GetAll(ctx)
	if err != nil {
		allCards = []*models.Card{} // Continue without bonuses if error
	}

	// Calculate card bonus
	cardBonus := h.calculateCardBonusWithCollection(ctx, userID, scenario, userCards, allCards)

	// Calculate rewards with card bonus
	rewards := calculateRewardsWithBonus(rarity, success, cardBonus)

	// Update user balance and work timestamp
    user, err := h.bot.UserRepository.GetByDiscordID(ctx, userID)
    if err != nil {
        _, uerr := e.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr("❌ Failed to process rewards")})
        return uerr
    }

	// Update balance
    if err := h.bot.UserRepository.UpdateBalance(ctx, user.DiscordID, rewards.Flakes); err != nil {
        _, uerr := e.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr("❌ Failed to update balance")})
        return uerr
    }

	// Update vials (you'll need to add this method to UserRepository)
	// For now, we'll skip vials update

	// Update work timestamp
    if err := h.bot.UserRepository.UpdateLastWork(ctx, user.DiscordID); err != nil {
        _, uerr := e.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr("❌ Failed to update work timestamp")})
        return uerr
    }

	// Track effect progress for Youth Youth By Young
	if h.bot.EffectManager != nil {
		go h.bot.EffectManager.UpdateEffectProgress(context.Background(), user.DiscordID, "youthyouth", 1)
	}

	// Process item drops
	if len(rewards.ItemDrops) > 0 {
		for _, itemID := range rewards.ItemDrops {
			if err := h.bot.ItemRepository.AddUserItem(ctx, user.DiscordID, itemID, 1); err != nil {
				// Log error but don't fail the whole operation
				fmt.Printf("Failed to add item %s to user %s: %v\n", itemID, user.DiscordID, err)
			}
		}
	}

	// Create result embed with card bonus info
	embed := h.createWorkResultEmbedWithBonus(success, rarity, rewards, cardBonus, scenario)

    _, uerr := e.UpdateInteractionResponse(discord.MessageUpdate{Embeds: &[]discord.Embed{embed}, Components: &[]discord.ContainerComponent{}})
    return uerr
}

func calculateRewards(rarity JobRarity, success bool) WorkRewards {
	if !success {
		// Failed jobs give minimal rewards
		return WorkRewards{
			Flakes: int64(5 + rand.Intn(10)),
			Vials:  int64(2 + rand.Intn(5)),
			XP:     int64(2 + rand.Intn(5)),
		}
	}

	// Base rewards based on rarity (lowered for better economy balance)
	var baseFlakes, baseVials, baseXP int64
	switch rarity {
	case JobRarity1Star:
		baseFlakes = 30
		baseVials = 15
		baseXP = 10
	case JobRarity2Star:
		baseFlakes = 60
		baseVials = 30
		baseXP = 20
	case JobRarity3Star:
		baseFlakes = 120
		baseVials = 60
		baseXP = 35
	case JobRarity4Star:
		baseFlakes = 250
		baseVials = 125
		baseXP = 60
	case JobRarity5Star:
		baseFlakes = 500
		baseVials = 250
		baseXP = 100
	}

	// Add some variance
	rewards := WorkRewards{
		Flakes: baseFlakes + int64(rand.Intn(int(baseFlakes/2))),
		Vials:  baseVials + int64(rand.Intn(int(baseVials/2))),
		XP:     baseXP + int64(rand.Intn(int(baseXP/2))),
	}

	// Item drops
	rewards.ItemDrops = calculateItemDrops(rarity)

	return rewards
}

func calculateItemDrops(rarity JobRarity) []string {
	items := []string{
		models.ItemBrokenDisc,
		models.ItemMicrophone,
		models.ItemForgottenSong,
	}

	var drops []string

	switch rarity {
	case JobRarity2Star:
		// 3% chance for one item
		if rand.Intn(100) < 3 {
			drops = append(drops, items[rand.Intn(len(items))])
		}
	case JobRarity3Star:
		// 9% chance for one item
		if rand.Intn(100) < 9 {
			drops = append(drops, items[rand.Intn(len(items))])
		}
	case JobRarity4Star:
		// Guaranteed one random item
		drops = append(drops, items[rand.Intn(len(items))])
	case JobRarity5Star:
		// Guaranteed one random item, 50% chance for a second
		drops = append(drops, items[rand.Intn(len(items))])
		if rand.Intn(100) < 50 {
			drops = append(drops, items[rand.Intn(len(items))])
		}
	}

	return drops
}

func createWorkResultEmbed(success bool, rarity JobRarity, rewards WorkRewards) discord.Embed {
	var title, description string
	var color int

	if success {
		title = "✅ Job Completed Successfully!"
		color = config.SuccessColor
		description = "Great work! You've earned:"
	} else {
		title = "❌ Job Failed"
		color = config.ErrorColor
		description = "Better luck next time! You still earned:"
	}

	embed := discord.NewEmbedBuilder().
		SetTitle(title).
		SetDescription(description).
		SetColor(color)

	// Add reward fields
	embed.AddField("❄️ Flakes", fmt.Sprintf("`%d`", rewards.Flakes), true)
	embed.AddField("🍷 Vials", fmt.Sprintf("`%d`", rewards.Vials), true)
	embed.AddField("✨ XP", fmt.Sprintf("`%d`", rewards.XP), true)

	// Add item drops if any
	if len(rewards.ItemDrops) > 0 {
		itemMap := map[string]string{
			models.ItemBrokenDisc:    "💿 Broken Disc",
			models.ItemMicrophone:    "🎤 Microphone",
			models.ItemForgottenSong: "📜 Forgotten Song",
		}

		var itemList []string
		for _, itemID := range rewards.ItemDrops {
			if name, ok := itemMap[itemID]; ok {
				itemList = append(itemList, name)
			}
		}

		embed.AddField("🎁 Bonus Items", strings.Join(itemList, "\n"), false)
	}

	// Add rarity info
	stars := strings.Repeat("⭐", int(rarity))
	embed.SetFooter(fmt.Sprintf("Job Rarity: %s", stars), "") // Cooldown disabled for testing

	return embed.Build()
}

// Calculate rewards with card bonus applied
func calculateRewardsWithBonus(rarity JobRarity, success bool, cardBonus CardBonus) WorkRewards {
	rewards := calculateRewards(rarity, success)

	if cardBonus.CombinedMultiplier > 1.0 {
		// Apply combined multiplier to base rewards
		rewards.Flakes = int64(float64(rewards.Flakes) * cardBonus.CombinedMultiplier)
		rewards.Vials = int64(float64(rewards.Vials) * cardBonus.CombinedMultiplier)
		rewards.XP = int64(float64(rewards.XP) * cardBonus.CombinedMultiplier)
	}

	return rewards
}

// Create result embed with card bonus information
func (h *WorkHandler) createWorkResultEmbedWithBonus(success bool, rarity JobRarity, rewards WorkRewards, cardBonus CardBonus, scenario EnhancedJobScenario) discord.Embed {
	var title string
	var color int

	if success {
		title = "✅ Job Completed!"
		color = config.SuccessColor
	} else {
		title = "❌ Job Failed"
		color = config.ErrorColor
	}

	embed := discord.NewEmbedBuilder().
		SetTitle(title).
		SetColor(color)

	// Add rewards with multiplier in a clean format
	hasBonus := cardBonus.CombinedMultiplier > 1.0 && success

	rewardText := fmt.Sprintf("❄️ **%d** Flakes", rewards.Flakes)
	if hasBonus {
		rewardText = fmt.Sprintf("❄️ **%d** Flakes ×%.1f", rewards.Flakes, cardBonus.CombinedMultiplier)
	}

	vialText := fmt.Sprintf("🍷 **%d** Vials", rewards.Vials)
	if hasBonus {
		vialText = fmt.Sprintf("🍷 **%d** Vials ×%.1f", rewards.Vials, cardBonus.CombinedMultiplier)
	}

	xpText := fmt.Sprintf("✨ **%d** XP", rewards.XP)
	if hasBonus {
		xpText = fmt.Sprintf("✨ **%d** XP ×%.1f", rewards.XP, cardBonus.CombinedMultiplier)
	}

	// Build description with rewards
	var description strings.Builder
	description.WriteString(rewardText + "\n")
	description.WriteString(vialText + "\n")
	description.WriteString(xpText)

	// Add item drops if any
	if len(rewards.ItemDrops) > 0 {
		description.WriteString("\n\n**🎁 Bonus Items**\n")
		itemMap := map[string]string{
			models.ItemBrokenDisc:    "💿 Broken Disc",
			models.ItemMicrophone:    "🎤 Microphone",
			models.ItemForgottenSong: "📜 Forgotten Song",
		}

		for _, itemID := range rewards.ItemDrops {
			if name, ok := itemMap[itemID]; ok {
				description.WriteString(name + "\n")
			}
		}
	}

	embed.SetDescription(description.String())

	// Add bonus information if job was successful
	if success {
		// Collection bonus field
		if cardBonus.CollectionName != "" {
			collectionIcon := "❌"
			collectionText := fmt.Sprintf("%d/%d cards", cardBonus.CollectionCardCount, 3)
			if cardBonus.HasCollectionBonus {
				collectionIcon = "✅"
				collectionText = fmt.Sprintf("%d cards (×%.1f)", cardBonus.CollectionCardCount, cardBonus.CollectionMultiplier)
			}
			embed.AddField(
				fmt.Sprintf("%s %s", collectionIcon, cardBonus.CollectionName),
				collectionText,
				true,
			)
		}

		// Tag bonus field
		tagIcon := "❌"
		tagText := fmt.Sprintf("%d cards", cardBonus.TagMatchCount)
		if cardBonus.HasTagBonus {
			tagIcon = "✅"
			tagText = fmt.Sprintf("%d cards (×%.1f)", cardBonus.TagMatchCount, cardBonus.TagMultiplier)
		}
		embed.AddField(
			fmt.Sprintf("%s Tags", tagIcon),
			tagText,
			true,
		)

		// Total multiplier field if any bonus applied
		if hasBonus {
			embed.AddField(
				"🎯 Total",
				fmt.Sprintf("×%.1f", cardBonus.CombinedMultiplier),
				true,
			)
		}
	}

	// Add rarity in footer
	stars := strings.Repeat("⭐", int(rarity))
	footerText := fmt.Sprintf("%s", stars)
	if !success {
		footerText += " | Better luck next time!"
	}
	embed.SetFooter(footerText, "")

	return embed.Build()
}

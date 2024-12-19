package commands

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Work = discord.SlashCommandCreate{
	Name:        "work",
	Description: "💼 Work in the K-pop industry to earn rewards",
}

type WorkSession struct {
	JobType    string
	Difficulty int
	Rewards    struct {
		Flakes int64
		Vials  int64
		XP     int64
	}
}

type WorkHandler struct {
	bot *bottemplate.Bot
}

func NewWorkHandler(b *bottemplate.Bot) *WorkHandler {
	return &WorkHandler{bot: b}
}

const workCooldown = 5 * time.Minute

func (h *WorkHandler) HandleWork(e *handler.CommandEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check cooldown
	user, err := h.bot.UserRepository.GetByDiscordID(ctx, e.User().ID.String())
	if err != nil {
		return utils.EH.CreateErrorEmbed(e, "Failed to fetch user data")
	}

	if time.Since(user.LastWork) < workCooldown {
		remaining := time.Until(user.LastWork.Add(workCooldown)).Round(time.Second)
		return utils.EH.CreateErrorEmbed(e, fmt.Sprintf("You need to rest for %s before working again!", remaining))
	}

	// Generate random job
	session := generateWorkSession()

	// Create initial job offer embed
	embed := createJobOfferEmbed(session)
	components := createJobComponents(session)

	return e.CreateMessage(discord.MessageCreate{
		Embeds:     []discord.Embed{embed},
		Components: []discord.ContainerComponent{components},
	})
}

func generateWorkSession() WorkSession {
	jobs := []string{
		"Trainee",
		"Backup Dancer",
		"Vocal Coach",
		"Choreographer",
		"Studio Engineer",
	}

	session := WorkSession{
		JobType:    jobs[rand.Intn(len(jobs))],
		Difficulty: rand.Intn(3) + 1, // 1-3 difficulty
	}

	// Calculate base rewards
	baseReward := int64(50 + (25 * session.Difficulty))
	session.Rewards.Flakes = baseReward * 2
	session.Rewards.Vials = baseReward / 2
	session.Rewards.XP = baseReward / 3

	return session
}

func createJobOfferEmbed(session WorkSession) discord.Embed {
	var description strings.Builder
	description.WriteString("```ansi\n")
	description.WriteString(fmt.Sprintf("\x1b[1;36mPosition:\x1b[0m %s\n", session.JobType))
	description.WriteString(fmt.Sprintf("\x1b[1;33mDifficulty:\x1b[0m %s\n", strings.Repeat("⭐", session.Difficulty)))
	description.WriteString("\n\x1b[1;32mPotential Rewards:\x1b[0m\n")
	description.WriteString(fmt.Sprintf("❄️ %d Flakes\n", session.Rewards.Flakes))
	description.WriteString(fmt.Sprintf("🍷 %d Vials\n", session.Rewards.Vials))
	description.WriteString(fmt.Sprintf("✨ %d XP\n", session.Rewards.XP))
	description.WriteString("```")

	return discord.NewEmbedBuilder().
		SetTitle("💼 K-pop Industry Job Offer").
		SetDescription(description.String()).
		SetColor(0x2b2d31).
		Build()
}

func createJobComponents(session WorkSession) discord.ContainerComponent {
	buttons := []discord.InteractiveComponent{
		discord.NewSuccessButton("Accept Job", fmt.Sprintf("work/accept/%s", session.JobType)),
		discord.NewDangerButton("Decline", "work/decline"),
	}
	return discord.NewActionRow(buttons...)
}

func (h *WorkHandler) HandleComponent(e *handler.ComponentEvent) error {
	parts := strings.Split(e.Data.CustomID(), "/")
	if len(parts) < 2 {
		return e.UpdateMessage(discord.MessageUpdate{
			Content: utils.Ptr("❌ Invalid interaction"),
		})
	}

	action := parts[1]
	switch action {
	case "accept":
		if len(parts) < 3 {
			return e.UpdateMessage(discord.MessageUpdate{
				Content: utils.Ptr("❌ Invalid job type"),
			})
		}
		return h.handleJobAccept(e, parts[2])
	case "decline":
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("💼 Maybe next time!"),
			Components: &[]discord.ContainerComponent{},
		})
	case "game":
		if len(parts) < 3 {
			return e.UpdateMessage(discord.MessageUpdate{
				Content: utils.Ptr("❌ Invalid game action"),
			})
		}
		return h.handleGameAction(e, parts[2:])
	default:
		return e.UpdateMessage(discord.MessageUpdate{
			Content: utils.Ptr("❌ Invalid action"),
		})
	}
}

func (h *WorkHandler) handleJobAccept(e *handler.ComponentEvent, jobType string) error {
	var gameEmbed discord.Embed
	var gameComponents []discord.ContainerComponent

	userID := e.User().ID.String()

	switch jobType {
	case "Trainee":
		gameEmbed, gameComponents = createTrainingGameEmbed(userID, e)
	case "Backup Dancer":
		gameEmbed, gameComponents = createDanceGameEmbed(userID, e)
	case "Vocal Coach":
		gameEmbed, gameComponents = createVocalGameEmbed(userID, e)
	case "Studio Engineer":
		gameEmbed, gameComponents = createMixingGameEmbed(userID, e)
	default:
		gameEmbed, gameComponents = createTrainingGameEmbed(userID, e)
	}

	return e.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{gameEmbed},
		Components: &gameComponents,
	})
}

func createTrainingGameEmbed(userID string, e *handler.ComponentEvent) (discord.Embed, []discord.ContainerComponent) {
	// K-pop Dance Practice patterns
	patterns := []string{"🕺", "💃", "👋", "💫"}
	state := createGameState("training", 5, len(patterns))
	state.Moves = patterns
	state.ShowSequence = true

	buttons := make([]discord.InteractiveComponent, len(patterns))
	for i, pattern := range patterns {
		buttons[i] = discord.NewSecondaryButton(pattern, fmt.Sprintf("work/game/training/%d", i))
	}

	components := []discord.ContainerComponent{discord.NewActionRow(buttons...)}
	state.Buttons = components

	sequence := make([]string, len(state.Sequence))
	for i, idx := range state.Sequence {
		sequence[i] = patterns[idx]
	}

	instructions := `🎵 K-pop Dance Practice

Learn this dance sequence:
%s

Each move represents:
🕺 - Basic Step
💃 - Spin Move
👋 - Wave Motion
💫 - Special Move

The sequence will disappear in 5 seconds!`

	embed := discord.NewEmbedBuilder().
		SetTitle("🎭 K-pop Dance Practice").
		SetDescription(fmt.Sprintf(instructions, strings.Join(sequence, " "))).
		SetColor(0x2b2d31).
		Build()

	// Store initial state
	stateMutex.Lock()
	gameStates[userID] = state
	stateMutex.Unlock()

	// Start goroutine to hide sequence
	go func(eventRef *handler.ComponentEvent, gameState *GameState) {
		time.Sleep(5 * time.Second)
		stateMutex.Lock()
		if state, exists := gameStates[userID]; exists && state == gameState {
			state.ShowSequence = false
			eventRef.UpdateMessage(discord.MessageUpdate{
				Embeds: &[]discord.Embed{
					discord.NewEmbedBuilder().
						SetTitle("🎭 K-pop Dance Practice").
						SetDescription("Time to perform the dance sequence!").
						SetColor(0x2b2d31).
						Build(),
				},
				Components: &state.Buttons,
			})
		}
		stateMutex.Unlock()
	}(e, state)

	return embed, components
}

func createDanceGameEmbed(userID string, e *handler.ComponentEvent) (discord.Embed, []discord.ContainerComponent) {
	moves := []string{"👆", "👇", "👈", "👉"}
	state := createGameState("dance", 4, len(moves))
	state.Moves = moves
	state.ShowSequence = true

	buttons := make([]discord.InteractiveComponent, len(moves))
	for i, move := range moves {
		buttons[i] = discord.NewSecondaryButton(move, fmt.Sprintf("work/game/dance/%d", i))
	}

	components := []discord.ContainerComponent{discord.NewActionRow(buttons...)}
	state.Buttons = components

	stateMutex.Lock()
	gameStates[userID] = state
	stateMutex.Unlock()

	sequence := make([]string, len(state.Sequence))
	for i, idx := range state.Sequence {
		sequence[i] = moves[idx]
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("🎵 Dance Practice").
		SetDescription(fmt.Sprintf("Memorize this dance sequence:\n%s\n\nThe sequence will disappear in 5 seconds!",
			strings.Join(sequence, " "))).
		SetColor(0x2b2d31).
		Build()

	// Start goroutine to hide sequence after 5 seconds
	go func(event *handler.ComponentEvent) {
		time.Sleep(5 * time.Second)
		stateMutex.Lock()
		if state, exists := gameStates[userID]; exists {
			state.ShowSequence = false
			// Add message update using the event
			event.UpdateMessage(discord.MessageUpdate{
				Content:    utils.Ptr("Time to repeat the sequence!"),
				Components: &state.Buttons,
			})
		}
		stateMutex.Unlock()
	}(e)

	return embed, components
}

func createVocalGameEmbed(userID string, e *handler.ComponentEvent) (discord.Embed, []discord.ContainerComponent) {
	notes := []string{"🔉 Low", "🔊 Mid", "📢 High"}
	state := createGameState("vocal", 4, len(notes))
	state.Moves = notes
	state.ShowSequence = true

	buttons := make([]discord.InteractiveComponent, len(notes))
	for i, note := range notes {
		buttons[i] = discord.NewSecondaryButton(note, fmt.Sprintf("work/game/vocal/%d", i))
	}

	components := []discord.ContainerComponent{discord.NewActionRow(buttons...)}
	state.Buttons = components

	stateMutex.Lock()
	gameStates[userID] = state
	stateMutex.Unlock()

	sequence := make([]string, len(state.Sequence))
	for i, idx := range state.Sequence {
		sequence[i] = notes[idx]
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("🎤 Vocal Training").
		SetDescription(fmt.Sprintf("Follow this note sequence:\n%s\n\nThe sequence will disappear in 5 seconds!",
			strings.Join(sequence, " → "))).
		SetColor(0x2b2d31).
		Build()

	// Start goroutine to hide sequence after 5 seconds
	go func(event *handler.ComponentEvent) {
		time.Sleep(5 * time.Second)
		stateMutex.Lock()
		if state, exists := gameStates[userID]; exists {
			state.ShowSequence = false
			// Update message without sequence
			event.UpdateMessage(discord.MessageUpdate{
				Content:    utils.Ptr("Time to repeat the sequence!"),
				Components: &state.Buttons,
			})
		}
		stateMutex.Unlock()
	}(e)

	return embed, components
}

func createMixingGameEmbed(userID string, e *handler.ComponentEvent) (discord.Embed, []discord.ContainerComponent) {
	controls := []string{"🎚️ Bass", "🔊 Volume", "🎛️ Treble", "🎮 Effects"}
	state := createGameState("mix", 4, len(controls))
	state.Moves = controls
	state.ShowSequence = true

	buttons := make([]discord.InteractiveComponent, len(controls))
	for i, control := range controls {
		buttons[i] = discord.NewSecondaryButton(control, fmt.Sprintf("work/game/mix/%d", i))
	}

	components := []discord.ContainerComponent{discord.NewActionRow(buttons...)}
	state.Buttons = components

	stateMutex.Lock()
	gameStates[userID] = state
	stateMutex.Unlock()

	sequence := make([]string, len(state.Sequence))
	for i, idx := range state.Sequence {
		sequence[i] = controls[idx]
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("🎛️ Studio Mixing").
		SetDescription(fmt.Sprintf("Adjust these controls in order:\n%s\n\nThe sequence will disappear in 5 seconds!",
			strings.Join(sequence, " → "))).
		SetColor(0x2b2d31).
		Build()

	// Start goroutine to hide sequence after 5 seconds
	go func() {
		time.Sleep(5 * time.Second)
		stateMutex.Lock()
		if state, exists := gameStates[userID]; exists {
			state.ShowSequence = false
			// Update the message to hide sequence
			e.UpdateMessage(discord.MessageUpdate{
				Content:    utils.Ptr("Time to repeat the sequence!"),
				Components: &state.Buttons,
			})
		}
		stateMutex.Unlock()
	}()

	return embed, components
}

type GameState struct {
	Sequence     []int
	Type         string
	Buttons      []discord.ContainerComponent
	Moves        []string
	ShowSequence bool
	Progress     []string
}

var (
	gameStates = make(map[string]*GameState) // Map of userID -> GameState
	stateMutex sync.RWMutex
)

func (h *WorkHandler) handleGameAction(e *handler.ComponentEvent, parts []string) error {
	if len(parts) < 2 {
		return e.UpdateMessage(discord.MessageUpdate{
			Content: utils.Ptr("❌ Invalid game action"),
		})
	}

	userID := e.User().ID.String()
	gameType := parts[0]
	choice, err := strconv.Atoi(parts[1])
	if err != nil {
		return e.UpdateMessage(discord.MessageUpdate{
			Content: utils.Ptr("❌ Invalid choice"),
		})
	}

	stateMutex.Lock()
	defer stateMutex.Unlock()

	state, exists := gameStates[userID]
	if !exists || state.Type != gameType {
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Game session expired - Please start a new job"),
			Components: &[]discord.ContainerComponent{},
		})
	}

	correct := handleGameChoice(state, choice)
	remainingCount := len(state.Sequence)

	if correct {
		// Add the correct move to progress
		state.Progress = append(state.Progress, state.Moves[choice])

		if remainingCount == 0 {
			delete(gameStates, userID)
			return h.handleSuccess(e)
		}

		// Build progress display
		var progressBuilder strings.Builder
		progressBuilder.WriteString("Your progress:\n\n")

		// Show completed moves with numbers
		for i, move := range state.Progress {
			progressBuilder.WriteString(fmt.Sprintf("%d. ✅ %s\n", i+1, move))
		}

		// Show next required move
		if remainingCount > 0 {
			progressBuilder.WriteString(fmt.Sprintf("\nNext move (%d/%d):\n❓ ???",
				len(state.Progress)+1,
				len(state.Progress)+remainingCount))
		}

		// Add game-specific UI elements
		switch state.Type {
		case "vocal":
			progressBuilder.WriteString("\n\n🎤 Keep the rhythm going!")
		case "dance":
			progressBuilder.WriteString("\n\n💃 Keep dancing!")
		case "mix":
			progressBuilder.WriteString("\n\n🎛️ Keep mixing!")
		case "choreo":
			progressBuilder.WriteString("\n\n🎭 Keep the flow!")
		}

		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr(progressBuilder.String()),
			Components: &state.Buttons,
			Embeds:     &[]discord.Embed{},
		})
	}

	// Handle incorrect move
	delete(gameStates, userID)
	return e.UpdateMessage(discord.MessageUpdate{
		Content: utils.Ptr(fmt.Sprintf("❌ Game Over!\nIncorrect move! The sequence was:\n%s",
			strings.Join(state.Moves, " → "))),
		Components: &[]discord.ContainerComponent{},
	})
}

func handleGameChoice(state *GameState, choice int) bool {
	if len(state.Sequence) == 0 {
		return false
	}

	expectedChoice := state.Sequence[0]
	if choice != expectedChoice {
		return false
	}

	// Remove the first element from sequence
	state.Sequence = state.Sequence[1:]
	return true
}

func createGameState(gameType string, sequenceLength int, maxChoice int) *GameState {
	sequence := make([]int, sequenceLength)
	for i := range sequence {
		sequence[i] = rand.Intn(maxChoice)
	}
	return &GameState{
		Sequence: sequence,
		Type:     gameType,
		Progress: make([]string, 0, sequenceLength),
	}
}

func (h *WorkHandler) handleSuccess(e *handler.ComponentEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := h.bot.UserRepository.GetByDiscordID(ctx, e.User().ID.String())
	if err != nil {
		return e.UpdateMessage(discord.MessageUpdate{
			Content: utils.Ptr("❌ Failed to process rewards"),
		})
	}

	// Calculate rewards
	flakes := int64(rand.Intn(100) + 50)
	vials := int64(rand.Intn(20) + 10)
	xp := int64(rand.Intn(30) + 15)

	// Update balance
	if err := h.bot.UserRepository.UpdateBalance(ctx, user.DiscordID, flakes); err != nil {
		return e.UpdateMessage(discord.MessageUpdate{
			Content: utils.Ptr("❌ Failed to update balance: " + err.Error()),
		})
	}

	// Update work timestamp
	if err := h.bot.UserRepository.UpdateLastWork(ctx, user.DiscordID); err != nil {
		fmt.Printf("Failed to update work timestamp: %v\n", err)
		return e.UpdateMessage(discord.MessageUpdate{
			Content: utils.Ptr("❌ Something went wrong while updating your work time. Please try again."),
		})
	}

	// Create an interactive embed for rewards
	embed := discord.NewEmbedBuilder().
		SetTitle("🎉 Great work!").
		SetDescription("Here are your rewards for completing the job:").
		SetColor(0x2ecc71).
		AddFields(
			discord.EmbedField{
				Name:   "❄️ Flakes",
				Value:  fmt.Sprintf("`%d`", flakes),
				Inline: utils.Ptr(true),
			},
			discord.EmbedField{
				Name:   "🍷 Vials",
				Value:  fmt.Sprintf("`%d`", vials),
				Inline: utils.Ptr(true),
			},
			discord.EmbedField{
				Name:   "✨ XP",
				Value:  fmt.Sprintf("`%d`", xp),
				Inline: utils.Ptr(true),
			},
		).
		SetFooter("Come back in 5 minutes to work again!", "").
		SetTimestamp(time.Now()).
		Build()

	// Create buttons for next actions
	buttons := []discord.InteractiveComponent{
		discord.NewPrimaryButton("📊 View Stats", "stats/view"),
		discord.NewSecondaryButton("💼 Work Again", "work/new"),
	}

	return e.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &[]discord.ContainerComponent{discord.NewActionRow(buttons...)},
	})
}

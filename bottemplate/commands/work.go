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

const workCooldown = 10 * time.Second

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
	patterns := []string{"🕺", "💃", "👋", "💫"}
	state := createGameState("training", 4, len(patterns))
	state.Moves = patterns
	state.ShowSequence = true

	// Create two rows of buttons for better mobile layout
	buttonsRow1 := make([]discord.InteractiveComponent, 2)
	buttonsRow2 := make([]discord.InteractiveComponent, 2)

	for i := 0; i < 2; i++ {
		buttonsRow1[i] = discord.NewSecondaryButton(patterns[i], fmt.Sprintf("work/game/training/%d", i))
		buttonsRow2[i] = discord.NewSecondaryButton(patterns[i+2], fmt.Sprintf("work/game/training/%d", i+2))
	}

	components := []discord.ContainerComponent{
		discord.NewActionRow(buttonsRow1...),
		discord.NewActionRow(buttonsRow2...),
	}
	state.Buttons = components

	sequence := make([]string, len(state.Sequence))
	for i, idx := range state.Sequence {
		sequence[i] = patterns[idx]
	}

	embed := createGameEmbed("🎭 Dance Practice", sequence, "Remember the moves!", patterns)

	// Store initial state
	stateMutex.Lock()
	gameStates[userID] = state
	stateMutex.Unlock()

	go hideSequence(e, state, userID, "🎭 Dance Practice")

	return embed, components
}

func createDanceGameEmbed(userID string, e *handler.ComponentEvent) (discord.Embed, []discord.ContainerComponent) {
	moves := []string{"👆", "👇", "👈", "👉"}
	state := createGameState("dance", 4, len(moves))
	state.Moves = moves
	state.ShowSequence = true

	// Create two rows for mobile layout
	buttonsRow1 := make([]discord.InteractiveComponent, 2)
	buttonsRow2 := make([]discord.InteractiveComponent, 2)

	for i := 0; i < 2; i++ {
		buttonsRow1[i] = discord.NewSecondaryButton(moves[i], fmt.Sprintf("work/game/dance/%d", i))
		buttonsRow2[i] = discord.NewSecondaryButton(moves[i+2], fmt.Sprintf("work/game/dance/%d", i+2))
	}

	components := []discord.ContainerComponent{
		discord.NewActionRow(buttonsRow1...),
		discord.NewActionRow(buttonsRow2...),
	}
	state.Buttons = components

	sequence := make([]string, len(state.Sequence))
	for i, idx := range state.Sequence {
		sequence[i] = moves[idx]
	}

	embed := createGameEmbed("💃 Choreography", sequence, "Follow the arrows!", moves)

	stateMutex.Lock()
	gameStates[userID] = state
	stateMutex.Unlock()

	go hideSequence(e, state, userID, "💃 Choreography")

	return embed, components
}

func createVocalGameEmbed(userID string, e *handler.ComponentEvent) (discord.Embed, []discord.ContainerComponent) {
	notes := []string{"🔉", "🔊", "📢"}
	state := createGameState("vocal", 4, len(notes))
	state.Moves = notes
	state.ShowSequence = true

	// Single row for 3 buttons
	buttons := make([]discord.InteractiveComponent, len(notes))
	for i, note := range notes {
		buttons[i] = discord.NewSecondaryButton(note, fmt.Sprintf("work/game/vocal/%d", i))
	}

	components := []discord.ContainerComponent{discord.NewActionRow(buttons...)}
	state.Buttons = components

	sequence := make([]string, len(state.Sequence))
	for i, idx := range state.Sequence {
		sequence[i] = notes[idx]
	}

	embed := createGameEmbed("🎤 Vocal Training", sequence, "Match the notes!", notes)

	stateMutex.Lock()
	gameStates[userID] = state
	stateMutex.Unlock()

	go hideSequence(e, state, userID, "🎤 Vocal Training")

	return embed, components
}

func createMixingGameEmbed(userID string, e *handler.ComponentEvent) (discord.Embed, []discord.ContainerComponent) {
	controls := []string{"🎚️", "🔊", "🎛️", "🎮"}
	state := createGameState("mix", 4, len(controls))
	state.Moves = controls
	state.ShowSequence = true

	// Create two rows for mobile layout
	buttonsRow1 := make([]discord.InteractiveComponent, 2)
	buttonsRow2 := make([]discord.InteractiveComponent, 2)

	for i := 0; i < 2; i++ {
		buttonsRow1[i] = discord.NewSecondaryButton(controls[i], fmt.Sprintf("work/game/mix/%d", i))
		buttonsRow2[i] = discord.NewSecondaryButton(controls[i+2], fmt.Sprintf("work/game/mix/%d", i+2))
	}

	components := []discord.ContainerComponent{
		discord.NewActionRow(buttonsRow1...),
		discord.NewActionRow(buttonsRow2...),
	}
	state.Buttons = components

	sequence := make([]string, len(state.Sequence))
	for i, idx := range state.Sequence {
		sequence[i] = controls[idx]
	}

	embed := createGameEmbed("🎛️ Studio Mixing", sequence, "Adjust the controls!", controls)

	stateMutex.Lock()
	gameStates[userID] = state
	stateMutex.Unlock()

	go hideSequence(e, state, userID, "🎛️ Studio Mixing")

	return embed, components
}

// Helper function to create consistent game embeds
func createGameEmbed(title string, sequence []string, instruction string, moves []string) discord.Embed {
	var description strings.Builder
	description.WriteString("```ansi\n")
	description.WriteString(fmt.Sprintf("\x1b[1;36m%s\x1b[0m\n\n", instruction))
	description.WriteString(fmt.Sprintf("\x1b[1;33mSequence:\x1b[0m %s\n\n", strings.Join(sequence, " ")))
	description.WriteString("\x1b[1;32mControls:\x1b[0m\n")
	for i, move := range moves {
		if i > 0 && i%2 == 0 {
			description.WriteString("\n")
		}
		description.WriteString(fmt.Sprintf("%s  ", move))
	}
	description.WriteString("\n```\n_Sequence disappears in 5s!_")

	return discord.NewEmbedBuilder().
		SetTitle(title).
		SetDescription(description.String()).
		SetColor(0x2b2d31).
		Build()
}

// Helper function to handle sequence hiding
func hideSequence(e *handler.ComponentEvent, state *GameState, userID string, title string) {
	time.Sleep(5 * time.Second)
	stateMutex.Lock()
	defer stateMutex.Unlock()

	if currentState, exists := gameStates[userID]; exists && currentState == state {
		state.ShowSequence = false
		e.UpdateMessage(discord.MessageUpdate{
			Embeds: &[]discord.Embed{
				discord.NewEmbedBuilder().
					SetTitle(title).
					SetDescription("Time to repeat the sequence!").
					SetColor(0x2b2d31).
					Build(),
			},
			Components: &state.Buttons,
		})
	}
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
			Content:    utils.Ptr("Game session expired - Please start a new job"),
			Components: &[]discord.ContainerComponent{},
		})
	}

	correct := handleGameChoice(state, choice)
	remainingCount := len(state.Sequence)

	if correct {
		state.Progress = append(state.Progress, state.Moves[choice])

		if remainingCount == 0 {
			delete(gameStates, userID)
			return h.handleSuccess(e)
		}

		// Build compact horizontal progress display
		progressStr := fmt.Sprintf("Progress: %s | %s",
			strings.Join(state.Progress, " "),
			strings.Repeat("□ ", remainingCount),
		)

		// Add game-specific message
		var gameMsg string
		switch state.Type {
		case "vocal":
			gameMsg = "🎤 Keep the rhythm!"
		case "dance":
			gameMsg = "💃 Keep dancing!"
		case "mix":
			gameMsg = "🎛️ Keep mixing!"
		case "training":
			gameMsg = "🎭 Keep practicing!"
		}

		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr(fmt.Sprintf("%s\n%s", progressStr, gameMsg)),
			Components: &state.Buttons,
			Embeds:     &[]discord.Embed{},
		})
	}

	// Handle incorrect move
	delete(gameStates, userID)

	// Build the actual sequence that was shown
	var actualSequence []string
	for _, idx := range state.Sequence {
		actualSequence = append(actualSequence, state.Moves[idx])
	}

	failEmbed := discord.NewEmbedBuilder().
		SetTitle("❌ Game Over!").
		SetDescription(fmt.Sprintf("**Sequence:** %s\n**Progress:** %s",
			strings.Join(actualSequence, " "),
			strings.Join(state.Progress, " "))).
		SetColor(0xff0000).
		Build()

	return e.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{failEmbed},
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
		SetFooter("Come back in 10 seconds to work again!", "").
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

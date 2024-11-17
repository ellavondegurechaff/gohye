package commands

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var ManageImages = discord.SlashCommandCreate{
	Name:        "manage-images",
	Description: "🖼️ Manage card images",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionSubCommand{
			Name:        "upload",
			Description: "Upload a new card image",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionInt{
					Name:        "card_id",
					Description: "The ID of the card",
					Required:    true,
				},
				discord.ApplicationCommandOptionAttachment{
					Name:        "image",
					Description: "The image file to upload (JPEG only)",
					Required:    true,
				},
			},
		},
		discord.ApplicationCommandOptionSubCommand{
			Name:        "update",
			Description: "Update an existing card image",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionInt{
					Name:        "card_id",
					Description: "The ID of the card",
					Required:    true,
				},
				discord.ApplicationCommandOptionAttachment{
					Name:        "image",
					Description: "The new image file (JPEG only)",
					Required:    true,
				},
			},
		},
		discord.ApplicationCommandOptionSubCommand{
			Name:        "verify",
			Description: "Verify image existence and sync",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionInt{
					Name:        "card_id",
					Description: "The ID of the card to verify",
					Required:    true,
				},
			},
		},
	},
}

func ManageImagesHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		start := time.Now()

		// Setup logger with request ID for tracing
		requestID := fmt.Sprintf("cmd-%s-%d", e.User().ID, time.Now().Unix())
		logger := slog.With(
			slog.String("request_id", requestID),
			slog.String("command", "manageimages"),
			slog.String("user_id", e.User().ID.String()),
		)

		logger.Info("Command started")

		// Increase timeout to 2 minutes
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Defer message with longer thinking time
		if err := e.DeferCreateMessage(true); err != nil {
			logger.Error("Failed to defer message", slog.String("error", err.Error()))
			return fmt.Errorf("failed to defer message: %w", err)
		}

		// Add timeout channel
		done := make(chan struct{})
		var result *services.ImageManagementResult
		var cmdErr error

		go func() {
			defer close(done)
			result, cmdErr = executeImageCommand(ctx, e, b, logger)
		}()

		// Wait for either completion or timeout
		select {
		case <-ctx.Done():
			logger.Error("Command timed out",
				slog.Duration("elapsed", time.Since(start)),
			)
			return createErrorEmbed(e, "Timeout", "The operation took too long to complete")
		case <-done:
			if cmdErr != nil {
				return createImageManagementResponse(e, result)
			}
			return createImageManagementResponse(e, result)
		}
	}
}

// Move the main command logic to a separate function
func executeImageCommand(ctx context.Context, e *handler.CommandEvent, b *bottemplate.Bot, logger *slog.Logger) (*services.ImageManagementResult, error) {
	data := e.SlashCommandInteractionData()
	subCmd := data.SubCommandName
	cardID := data.Int("card_id")

	logger.Info("Processing command",
		slog.String("subcommand", *subCmd),
		slog.Int64("card_id", int64(cardID)),
	)

	card, err := b.CardRepository.GetByID(ctx, int64(cardID))
	if err != nil {
		logger.Error("Failed to get card",
			slog.Int64("card_id", int64(cardID)),
			slog.String("error", err.Error()),
		)
		return nil, createErrorEmbed(e, "Error", fmt.Sprintf("Card with ID %d does not exist", cardID))
	}

	logger.Info("Card found",
		slog.String("card_name", card.Name),
		slog.String("collection", card.ColID),
		slog.Int("level", card.Level),
	)

	var operation services.ImageOperation
	var imageData []byte

	switch *subCmd {
	case "upload", "update":
		operation = services.ImageOperation(*subCmd)
		attachment := data.Attachment("image")

		logger.Info("Processing attachment",
			slog.String("filename", attachment.Filename),
			slog.String("url", attachment.URL),
			slog.Int("size", attachment.Size),
		)

		if !strings.HasSuffix(strings.ToLower(attachment.Filename), ".jpg") &&
			!strings.HasSuffix(strings.ToLower(attachment.Filename), ".jpeg") {
			logger.Error("Invalid file type", slog.String("filename", attachment.Filename))
			return nil, createErrorEmbed(e, "Invalid File", "Only JPEG images are supported")
		}

		imageData, err = downloadImage(attachment.URL)
		if err != nil {
			logger.Error("Failed to download image",
				slog.String("url", attachment.URL),
				slog.String("error", err.Error()),
			)
			return nil, createErrorEmbed(e, "Download Failed", fmt.Sprintf("Failed to download image: %v", err))
		}

		logger.Info("Image downloaded successfully",
			slog.Int("bytes", len(imageData)),
		)

	case "verify":
		operation = services.ImageOperationVerify
		logger.Info("Starting image verification",
			slog.String("card_name", card.Name),
			slog.String("collection", card.ColID),
			slog.Int("level", card.Level),
			slog.String("group_type", utils.GetGroupType(card.Tags)),
		)

	default:
		logger.Error("Invalid subcommand", slog.String("subcommand", *subCmd))
		return nil, createErrorEmbed(e, "Invalid Operation", "Unknown subcommand")
	}

	logger.Info("Calling ManageCardImage",
		slog.String("operation", string(operation)),
		slog.Int64("card_id", int64(cardID)),
	)

	result, err := b.SpacesService.ManageCardImage(ctx, operation, int64(cardID), imageData, card)
	if err != nil {
		logger.Error("ManageCardImage failed",
			slog.String("operation", string(operation)),
			slog.String("error", err.Error()),
		)

		if result == nil {
			logger.Info("Creating default error result")
			result = &services.ImageManagementResult{
				Operation:    operation,
				Success:      false,
				CardName:     card.Name,
				CollectionID: card.ColID,
				Level:        card.Level,
				ErrorMessage: err.Error(),
			}
		}
		return result, err
	}

	logger.Info("Operation successful",
		slog.String("operation", string(operation)),
		slog.String("url", result.URL),
	)

	return result, nil
}

func downloadImage(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func createImageManagementResponse(e *handler.CommandEvent, result *services.ImageManagementResult) error {
	var description strings.Builder
	description.WriteString("```md\n")
	description.WriteString("# Operation Details\n")
	description.WriteString(fmt.Sprintf("* Operation: %s\n", result.Operation))
	description.WriteString(fmt.Sprintf("* Card Name: %s\n", result.CardName))
	description.WriteString(fmt.Sprintf("* Collection: %s\n", result.CollectionID))
	description.WriteString(fmt.Sprintf("* Level: %d\n", result.Level))

	// Fix: Replace map lookup with direct string assignment
	statusText := "❌ Failed"
	if result.Success {
		statusText = "✅ Success"
	}
	description.WriteString(fmt.Sprintf("* Status: %s\n", statusText))

	if result.ErrorMessage != "" {
		description.WriteString(fmt.Sprintf("* Error: %s\n", result.ErrorMessage))
	}
	description.WriteString("```")

	if result.Success {
		description.WriteString(fmt.Sprintf("\n**Image URL:**\n%s", result.URL))
	}

	color := utils.ErrorColor
	if result.Success {
		color = utils.SuccessColor
	}

	now := time.Now()
	_, err := e.CreateFollowupMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Title:       fmt.Sprintf("🖼️ Image Management - %s", result.Operation),
			Description: description.String(),
			Color:       color,
			Image: &discord.EmbedResource{
				URL: result.URL,
			},
			Timestamp: &now,
			Footer: &discord.EmbedFooter{
				Text: "Image Management System",
			},
		}},
	})
	return err
}

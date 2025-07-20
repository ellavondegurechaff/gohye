package utils

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

// DiffPaginationConfig holds configuration for diff pagination
type DiffPaginationConfig struct {
	ItemsPerPage int
}

// DiffPaginationData holds the data needed for diff pagination
type DiffPaginationData struct {
	Items        []interface{}
	TotalItems   int
	UserID       string
	SubCommand   string
	TargetUserID string
	Query        string
	Title        string
}

// DiffPaginationHandler handles pagination logic specifically for diff command
type DiffPaginationHandler struct {
	Config       DiffPaginationConfig
	FormatItems  func(items []interface{}, page, totalPages int, data *DiffPaginationData) (discord.Embed, error)
	FormatCopy   func(items []interface{}, title string) string
	ValidateUser func(eventUserID, targetUserID string) bool
}

// NewDiffPaginationHandler creates a new diff pagination handler
func NewDiffPaginationHandler() *DiffPaginationHandler {
	return &DiffPaginationHandler{
		Config: DiffPaginationConfig{
			ItemsPerPage: config.CardsPerPage,
		},
		ValidateUser: func(eventUserID, targetUserID string) bool {
			return eventUserID == targetUserID
		},
	}
}

// HandleDiffPagination processes diff pagination component interactions
func (dph *DiffPaginationHandler) HandleDiffPagination(
	ctx context.Context,
	e *handler.ComponentEvent,
	getData func(ctx context.Context, userID, subCmd, targetUserID, query string) (*DiffPaginationData, error),
) error {
	data := e.Data.(discord.ButtonInteractionData)
	customID := data.CustomID()

	parts := strings.Split(customID, "/")
	if len(parts) < 7 {
		return nil
	}

	userID := parts[3]
	currentPage, err := strconv.Atoi(parts[4])
	if err != nil {
		return nil
	}

	subCmd := parts[5]
	targetUserID := parts[6]
	query := strings.Join(parts[7:], "/")

	// Validate user if validation function provided
	if dph.ValidateUser != nil && !dph.ValidateUser(e.User().ID.String(), userID) {
		return EH.CreateEphemeralError(e, "Only the command user can navigate through these cards.")
	}

	// Handle copy button
	if strings.Contains(customID, "/copy/") {
		if dph.FormatCopy == nil {
			return EH.CreateEphemeralError(e, "Copy functionality not available.")
		}

		paginationData, err := getData(ctx, userID, subCmd, targetUserID, query)
		if err != nil {
			return EH.CreateEphemeralError(e, "Failed to fetch data")
		}

		startIdx := currentPage * dph.Config.ItemsPerPage
		endIdx := min(startIdx+dph.Config.ItemsPerPage, len(paginationData.Items))

		if startIdx >= len(paginationData.Items) {
			return EH.CreateEphemeralError(e, "No items to copy")
		}

		copyText := dph.FormatCopy(paginationData.Items[startIdx:endIdx], paginationData.Title)
		return e.CreateMessage(discord.MessageCreate{
			Content: "```\n" + copyText + "```",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Get fresh data
	paginationData, err := getData(ctx, userID, subCmd, targetUserID, query)
	if err != nil {
		return EH.CreateEphemeralError(e, "Failed to fetch data")
	}

	if len(paginationData.Items) == 0 {
		return EH.CreateEphemeralError(e, "No items found")
	}

	totalPages := int(math.Ceil(float64(len(paginationData.Items)) / float64(dph.Config.ItemsPerPage)))

	// Calculate new page
	newPage := currentPage
	if strings.Contains(customID, "/next/") {
		newPage = (currentPage + 1) % totalPages
	} else if strings.Contains(customID, "/prev/") {
		newPage = (currentPage - 1 + totalPages) % totalPages
	}

	// Calculate slice boundaries for current page
	startIdx := newPage * dph.Config.ItemsPerPage
	endIdx := min(startIdx+dph.Config.ItemsPerPage, len(paginationData.Items))

	if startIdx >= len(paginationData.Items) {
		return EH.CreateEphemeralError(e, "Page not found")
	}

	// Slice items for current page only
	pageItems := paginationData.Items[startIdx:endIdx]

	// Create embed for new page
	embed, err := dph.FormatItems(pageItems, newPage, totalPages, paginationData)
	if err != nil {
		return EH.CreateEphemeralError(e, "Failed to format items")
	}

	// Create pagination components
	components := dph.CreateDiffPaginationComponents(newPage, totalPages, userID, subCmd, targetUserID, query)

	return e.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &components,
	})
}

// CreateDiffPaginationComponents creates the pagination buttons for diff command
func (dph *DiffPaginationHandler) CreateDiffPaginationComponents(page, totalPages int, userID, subCmd, targetUserID, query string) []discord.ContainerComponent {
	var components []discord.ContainerComponent

	if totalPages > 1 {
		var buttons []discord.InteractiveComponent

		// Previous button
		buttons = append(buttons, discord.NewSecondaryButton(
			"◀ Previous",
			fmt.Sprintf("/diff/prev/%s/%d/%s/%s/%s", userID, page, subCmd, targetUserID, query),
		))

		// Next button
		buttons = append(buttons, discord.NewSecondaryButton(
			"Next ▶",
			fmt.Sprintf("/diff/next/%s/%d/%s/%s/%s", userID, page, subCmd, targetUserID, query),
		))

		// Add copy button if format function exists
		if dph.FormatCopy != nil {
			buttons = append(buttons, discord.NewSecondaryButton(
				"📋 Copy Page",
				fmt.Sprintf("/diff/copy/%s/%d/%s/%s/%s", userID, page, subCmd, targetUserID, query),
			))
		}

		components = append(components, discord.NewActionRow(buttons...))
	}

	return components
}

// CreateInitialDiffPaginationEmbed creates the initial paginated embed for diff command
func (dph *DiffPaginationHandler) CreateInitialDiffPaginationEmbed(
	data *DiffPaginationData,
) (discord.Embed, []discord.ContainerComponent, error) {
	if len(data.Items) == 0 {
		return discord.Embed{}, nil, fmt.Errorf("no items found")
	}

	totalPages := int(math.Ceil(float64(len(data.Items)) / float64(dph.Config.ItemsPerPage)))

	// Get items for first page only
	pageItems := data.Items
	if len(data.Items) > dph.Config.ItemsPerPage {
		pageItems = data.Items[:dph.Config.ItemsPerPage]
	}

	embed, err := dph.FormatItems(pageItems, 0, totalPages, data)
	if err != nil {
		return discord.Embed{}, nil, err
	}

	components := dph.CreateDiffPaginationComponents(0, totalPages, data.UserID, data.SubCommand, data.TargetUserID, data.Query)

	return embed, components, nil
}

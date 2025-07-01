package utils

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

// PaginationConfig holds configuration for pagination
type PaginationConfig struct {
	ItemsPerPage int
	Prefix       string // e.g., "cards", "miss", "inventory"
}

// PaginationData holds the data needed for pagination
type PaginationData struct {
	Items      []interface{}
	TotalItems int
	UserID     string
	Query      string
}

// PaginationHandler handles common pagination logic
type PaginationHandler struct {
	Config       PaginationConfig
	FormatItems  func(items []interface{}, page, totalPages int, userID, query string) (discord.Embed, error)
	FormatCopy   func(items []interface{}) string
	ValidateUser func(eventUserID, targetUserID string) bool
}

// HandlePagination processes pagination component interactions
func (ph *PaginationHandler) HandlePagination(
	ctx context.Context,
	e *handler.ComponentEvent,
	getData func(userID, query string) (*PaginationData, error),
) error {
	data := e.Data.(discord.ButtonInteractionData)
	customID := data.CustomID()

	parts := strings.Split(customID, "/")
	if len(parts) < 5 {
		return nil
	}

	userID := parts[3]
	currentPage, err := strconv.Atoi(parts[4])
	if err != nil {
		return nil
	}

	query := ""
	if len(parts) > 5 {
		query = parts[5]
	}

	// Validate user if validation function provided
	if ph.ValidateUser != nil && !ph.ValidateUser(e.User().ID.String(), userID) {
		return EH.CreateEphemeralError(e, "Only the command user can navigate through these items.")
	}

	// Handle copy button
	if strings.Contains(customID, "/copy/") {
		if ph.FormatCopy == nil {
			return EH.CreateEphemeralError(e, "Copy functionality not available.")
		}

		paginationData, err := getData(userID, query)
		if err != nil {
			return EH.CreateEphemeralError(e, "Failed to fetch data")
		}

		startIdx := currentPage * ph.Config.ItemsPerPage
		endIdx := min(startIdx+ph.Config.ItemsPerPage, len(paginationData.Items))

		if startIdx >= len(paginationData.Items) {
			return EH.CreateEphemeralError(e, "No items to copy")
		}

		copyText := ph.FormatCopy(paginationData.Items[startIdx:endIdx])
		return e.CreateMessage(discord.MessageCreate{
			Content: "```\n" + copyText + "```",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Get fresh data
	paginationData, err := getData(userID, query)
	if err != nil {
		return EH.CreateEphemeralError(e, "Failed to fetch data")
	}

	if len(paginationData.Items) == 0 {
		return EH.CreateEphemeralError(e, "No items found")
	}

	totalPages := int(math.Ceil(float64(len(paginationData.Items)) / float64(ph.Config.ItemsPerPage)))

	// Calculate new page
	newPage := currentPage
	if strings.Contains(customID, "/next/") {
		newPage = (currentPage + 1) % totalPages
	} else if strings.Contains(customID, "/prev/") {
		newPage = (currentPage - 1 + totalPages) % totalPages
	}

	// Calculate slice boundaries for current page
	startIdx := newPage * ph.Config.ItemsPerPage
	endIdx := min(startIdx+ph.Config.ItemsPerPage, len(paginationData.Items))

	if startIdx >= len(paginationData.Items) {
		return EH.CreateEphemeralError(e, "Page not found")
	}

	// Slice items for current page only
	pageItems := paginationData.Items[startIdx:endIdx]

	// Create embed for new page
	embed, err := ph.FormatItems(pageItems, newPage, totalPages, userID, query)
	if err != nil {
		return EH.CreateEphemeralError(e, "Failed to format items")
	}

	// Create pagination components
	components := ph.CreatePaginationComponents(newPage, totalPages, userID, query)

	return e.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &components,
	})
}

// CreatePaginationComponents creates the pagination buttons
func (ph *PaginationHandler) CreatePaginationComponents(page, totalPages int, userID, query string) []discord.ContainerComponent {
	var components []discord.ContainerComponent

	if totalPages > 1 {
		queryParam := ""
		if query != "" {
			queryParam = "/" + query
		}

		var buttons []discord.InteractiveComponent

		// Previous button
		buttons = append(buttons, discord.NewSecondaryButton(
			"◀ Previous",
			fmt.Sprintf("/%s/prev/%s/%d%s", ph.Config.Prefix, userID, page, queryParam),
		))

		// Next button
		buttons = append(buttons, discord.NewSecondaryButton(
			"Next ▶",
			fmt.Sprintf("/%s/next/%s/%d%s", ph.Config.Prefix, userID, page, queryParam),
		))

		// Add copy button if format function exists
		if ph.FormatCopy != nil {
			buttons = append(buttons, discord.NewSecondaryButton(
				"📋 Copy Page",
				fmt.Sprintf("/%s/copy/%s/%d%s", ph.Config.Prefix, userID, page, queryParam),
			))
		}

		components = append(components, discord.NewActionRow(buttons...))
	}

	return components
}

// CreateInitialPaginationEmbed creates the initial paginated embed
func (ph *PaginationHandler) CreateInitialPaginationEmbed(
	items []interface{},
	userID, query string,
) (discord.Embed, []discord.ContainerComponent, error) {
	if len(items) == 0 {
		return discord.Embed{}, nil, fmt.Errorf("no items found")
	}

	totalPages := int(math.Ceil(float64(len(items)) / float64(ph.Config.ItemsPerPage)))

	// Slice items for first page only
	endIdx := min(ph.Config.ItemsPerPage, len(items))
	pageItems := items[0:endIdx]

	embed, err := ph.FormatItems(pageItems, 0, totalPages, userID, query)
	if err != nil {
		return discord.Embed{}, nil, err
	}

	components := ph.CreatePaginationComponents(0, totalPages, userID, query)

	return embed, components, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
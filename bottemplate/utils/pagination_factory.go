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

// ComponentIDParser defines the interface for parsing component IDs
type ComponentIDParser interface {
	Parse(customID string) (PaginationParams, error)
	BuildComponentID(prefix, action string, params PaginationParams) string
}

// PaginationParams contains all possible parameters for pagination
type PaginationParams struct {
	UserID         string
	Page           int
	Query          string
	SubCommand     string
	TargetUserID   string
	SortByProgress bool
	CompletedOnly  bool
}

// DataFetcher defines the interface for fetching paginated data
type DataFetcher interface {
	FetchData(ctx context.Context, params PaginationParams) ([]interface{}, error)
}

// ItemFormatter defines the interface for formatting items into embeds
type ItemFormatter interface {
	FormatItems(items []interface{}, page, totalPages int, params PaginationParams) (discord.Embed, error)
	FormatCopy(items []interface{}, params PaginationParams) string
}

// UserValidator defines the interface for validating user permissions
type UserValidator interface {
	ValidateUser(eventUserID string, params PaginationParams) bool
}

// PaginationFactoryConfig holds the configuration for pagination factory
type PaginationFactoryConfig struct {
	ItemsPerPage int
	Prefix       string
	Parser       ComponentIDParser
	Fetcher      DataFetcher
	Formatter    ItemFormatter
	Validator    UserValidator
}

// PaginationFactory creates unified pagination handlers
type PaginationFactory struct {
	config PaginationFactoryConfig
}

// NewPaginationFactory creates a new pagination factory
func NewPaginationFactory(config PaginationFactoryConfig) *PaginationFactory {
	return &PaginationFactory{
		config: config,
	}
}

// CreateHandler creates a unified pagination handler
func (pf *PaginationFactory) CreateHandler() handler.ComponentHandler {
	return func(e *handler.ComponentEvent) error {
		ctx := context.Background()
		data := e.Data.(discord.ButtonInteractionData)
		customID := data.CustomID()

		// Parse component ID
		params, err := pf.config.Parser.Parse(customID)
		if err != nil {
			return nil // Invalid component ID, ignore
		}

		// Validate user if validator provided
		if pf.config.Validator != nil && !pf.config.Validator.ValidateUser(e.User().ID.String(), params) {
			return EH.CreateEphemeralError(e, "Only the command user can navigate through these items.")
		}

		// Handle copy button
		if strings.Contains(customID, "/copy/") {
			return pf.handleCopyButton(ctx, e, params)
		}

		// Handle navigation
		return pf.handleNavigation(ctx, e, customID, params)
	}
}

// handleCopyButton handles copy button interactions
func (pf *PaginationFactory) handleCopyButton(ctx context.Context, e *handler.ComponentEvent, params PaginationParams) error {
	// Fetch data
	items, err := pf.config.Fetcher.FetchData(ctx, params)
	if err != nil {
		return EH.CreateEphemeralError(e, "Failed to fetch data")
	}

	// Calculate page items
	startIdx := params.Page * pf.config.ItemsPerPage
	endIdx := min(startIdx+pf.config.ItemsPerPage, len(items))

	if startIdx >= len(items) {
		return EH.CreateEphemeralError(e, "No items to copy")
	}

	// Format copy text
	copyText := pf.config.Formatter.FormatCopy(items[startIdx:endIdx], params)
	return e.CreateMessage(discord.MessageCreate{
		Content: "```\n" + copyText + "```",
		Flags:   discord.MessageFlagEphemeral,
	})
}

// handleNavigation handles navigation button interactions
func (pf *PaginationFactory) handleNavigation(ctx context.Context, e *handler.ComponentEvent, customID string, params PaginationParams) error {
	// Fetch fresh data
	items, err := pf.config.Fetcher.FetchData(ctx, params)
	if err != nil {
		return EH.CreateEphemeralError(e, "Failed to fetch data")
	}

	if len(items) == 0 {
		return EH.CreateEphemeralError(e, "No items found")
	}

	totalPages := int(math.Ceil(float64(len(items)) / float64(pf.config.ItemsPerPage)))

	// Calculate new page
	newPage := params.Page
	if strings.Contains(customID, "/next/") {
		newPage = (params.Page + 1) % totalPages
	} else if strings.Contains(customID, "/prev/") {
		newPage = (params.Page - 1 + totalPages) % totalPages
	}

	// Update params with new page
	newParams := params
	newParams.Page = newPage

	// Calculate page items
	startIdx := newPage * pf.config.ItemsPerPage
	endIdx := min(startIdx+pf.config.ItemsPerPage, len(items))
	
	if startIdx >= len(items) {
		startIdx = 0
		endIdx = min(pf.config.ItemsPerPage, len(items))
	}
	
	pageItems := items[startIdx:endIdx]
	
	// Create embed for new page
	embed, err := pf.config.Formatter.FormatItems(pageItems, newPage, totalPages, newParams)
	if err != nil {
		return EH.CreateEphemeralError(e, "Failed to format items")
	}

	// Create pagination components
	components := pf.createPaginationComponents(totalPages, newParams)

	return e.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &components,
	})
}

// createPaginationComponents creates the pagination buttons
func (pf *PaginationFactory) createPaginationComponents(totalPages int, params PaginationParams) []discord.ContainerComponent {
	var components []discord.ContainerComponent

	if totalPages > 1 {
		var buttons []discord.InteractiveComponent

		// Previous button
		prevID := pf.config.Parser.BuildComponentID(pf.config.Prefix, "prev", params)
		buttons = append(buttons, discord.NewSecondaryButton("◀ Previous", prevID))

		// Next button
		nextID := pf.config.Parser.BuildComponentID(pf.config.Prefix, "next", params)
		buttons = append(buttons, discord.NewSecondaryButton("Next ▶", nextID))

		// Copy button
		copyID := pf.config.Parser.BuildComponentID(pf.config.Prefix, "copy", params)
		buttons = append(buttons, discord.NewSecondaryButton("📋 Copy Page", copyID))

		components = append(components, discord.NewActionRow(buttons...))
	}

	return components
}

// CreateInitialPaginationEmbed creates the initial paginated embed
func (pf *PaginationFactory) CreateInitialPaginationEmbed(ctx context.Context, params PaginationParams) (discord.Embed, []discord.ContainerComponent, error) {
	// Fetch data
	items, err := pf.config.Fetcher.FetchData(ctx, params)
	if err != nil {
		return discord.Embed{}, nil, err
	}

	if len(items) == 0 {
		return discord.Embed{}, nil, fmt.Errorf("no items found")
	}

	totalPages := int(math.Ceil(float64(len(items)) / float64(pf.config.ItemsPerPage)))

	// Get items for first page only
	pageItems := items
	if len(items) > pf.config.ItemsPerPage {
		pageItems = items[:pf.config.ItemsPerPage]
	}

	// Create embed for first page
	embed, err := pf.config.Formatter.FormatItems(pageItems, 0, totalPages, params)
	if err != nil {
		return discord.Embed{}, nil, err
	}

	// Create pagination components
	components := pf.createPaginationComponents(totalPages, params)

	return embed, components, nil
}

// RegularParser implements ComponentIDParser for regular pagination format
// Format: /{prefix}/{action}/{userID}/{page}/{query?}
type RegularParser struct {
	prefix string
}

// NewRegularParser creates a new regular parser
func NewRegularParser(prefix string) *RegularParser {
	return &RegularParser{prefix: prefix}
}

// Parse parses a regular component ID
// Format: /{prefix}/{action}/{userID}/{page}/{query?}/{sortByProgress?}/{completedOnly?}
func (rp *RegularParser) Parse(customID string) (PaginationParams, error) {
	parts := strings.Split(customID, "/")
	if len(parts) < 5 {
		return PaginationParams{}, fmt.Errorf("invalid component ID format")
	}

	page, err := strconv.Atoi(parts[4])
	if err != nil {
		return PaginationParams{}, fmt.Errorf("invalid page number")
	}

	params := PaginationParams{
		UserID: parts[3],
		Page:   page,
	}

	if len(parts) > 5 && parts[5] != "" {
		params.Query = parts[5]
	}

	if len(parts) > 6 && parts[6] != "" {
		sortByProgress, _ := strconv.ParseBool(parts[6])
		params.SortByProgress = sortByProgress
	}

	if len(parts) > 7 && parts[7] != "" {
		completedOnly, _ := strconv.ParseBool(parts[7])
		params.CompletedOnly = completedOnly
	}

	return params, nil
}

// BuildComponentID builds a component ID for regular pagination
func (rp *RegularParser) BuildComponentID(prefix, action string, params PaginationParams) string {
	parts := []string{"", prefix, action, params.UserID, strconv.Itoa(params.Page)}
	
	// Add query if present
	if params.Query != "" {
		parts = append(parts, params.Query)
	} else {
		parts = append(parts, "")
	}
	
	// Add boolean flags if either is true
	if params.SortByProgress || params.CompletedOnly {
		parts = append(parts, strconv.FormatBool(params.SortByProgress))
		parts = append(parts, strconv.FormatBool(params.CompletedOnly))
	}
	
	return strings.Join(parts, "/")
}

// DiffParser implements ComponentIDParser for diff pagination format
// Format: /diff/{action}/{userID}/{page}/{subCmd}/{targetUserID}/{query}
type DiffParser struct{}

// NewDiffParser creates a new diff parser
func NewDiffParser() *DiffParser {
	return &DiffParser{}
}

// Parse parses a diff component ID
func (dp *DiffParser) Parse(customID string) (PaginationParams, error) {
	parts := strings.Split(customID, "/")
	if len(parts) < 7 {
		return PaginationParams{}, fmt.Errorf("invalid diff component ID format")
	}

	page, err := strconv.Atoi(parts[4])
	if err != nil {
		return PaginationParams{}, fmt.Errorf("invalid page number")
	}

	params := PaginationParams{
		UserID:       parts[3],
		Page:         page,
		SubCommand:   parts[5],
		TargetUserID: parts[6],
	}

	if len(parts) > 7 {
		params.Query = strings.Join(parts[7:], "/")
	}

	return params, nil
}

// BuildComponentID builds a component ID for diff pagination
func (dp *DiffParser) BuildComponentID(prefix, action string, params PaginationParams) string {
	if params.Query != "" {
		return fmt.Sprintf("/diff/%s/%s/%d/%s/%s/%s", action, params.UserID, params.Page, params.SubCommand, params.TargetUserID, params.Query)
	}
	return fmt.Sprintf("/diff/%s/%s/%d/%s/%s", action, params.UserID, params.Page, params.SubCommand, params.TargetUserID)
}

// Backwards compatibility adapters

// LegacyPaginationHandlerAdapter adapts the old PaginationHandler to the new factory system
type LegacyPaginationHandlerAdapter struct {
	handler *PaginationHandler
	getData func(userID, query string) (*PaginationData, error)
}

// NewLegacyPaginationHandlerAdapter creates an adapter for existing PaginationHandler
func NewLegacyPaginationHandlerAdapter(handler *PaginationHandler, getData func(userID, query string) (*PaginationData, error)) *LegacyPaginationHandlerAdapter {
	return &LegacyPaginationHandlerAdapter{
		handler: handler,
		getData: getData,
	}
}

// FetchData implements DataFetcher for legacy pagination
func (lpha *LegacyPaginationHandlerAdapter) FetchData(ctx context.Context, params PaginationParams) ([]interface{}, error) {
	data, err := lpha.getData(params.UserID, params.Query)
	if err != nil {
		return nil, err
	}
	return data.Items, nil
}

// FormatItems implements ItemFormatter for legacy pagination
func (lpha *LegacyPaginationHandlerAdapter) FormatItems(items []interface{}, page, totalPages int, params PaginationParams) (discord.Embed, error) {
	return lpha.handler.FormatItems(items, page, totalPages, params.UserID, params.Query)
}

// FormatCopy implements ItemFormatter for legacy pagination
func (lpha *LegacyPaginationHandlerAdapter) FormatCopy(items []interface{}, params PaginationParams) string {
	if lpha.handler.FormatCopy == nil {
		return ""
	}
	return lpha.handler.FormatCopy(items)
}

// ValidateUser implements UserValidator for legacy pagination
func (lpha *LegacyPaginationHandlerAdapter) ValidateUser(eventUserID string, params PaginationParams) bool {
	if lpha.handler.ValidateUser == nil {
		return true
	}
	return lpha.handler.ValidateUser(eventUserID, params.UserID)
}
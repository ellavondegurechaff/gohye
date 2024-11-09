package repositories

// SearchFilters defines the available filters for card searches
type SearchFilters struct {
	Name       string
	ID         int64
	Level      int
	Collection string
	Type       string
	Animated   bool
}

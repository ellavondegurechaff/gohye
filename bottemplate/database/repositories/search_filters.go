package repositories

import "time"

// SearchFilters defines the available filters for card searches
// Enhanced to support legacy system functionality while maintaining backward compatibility
type SearchFilters struct {
	// Original fields (maintained for backward compatibility)
	Name       string
	ID         int64
	Level      int
	Collection string
	Type       string
	Animated   bool
	
	// Enhanced filters matching utils.SearchFilters
	Query           string
	Levels          []int
	Collections     []string
	AntiCollections []string
	AntiLevels      []int
	Tags            []string
	AntiTags        []string
	
	// User-specific filters
	UserID           string
	Favorites        bool
	ExcludeFavorites bool
	LockedOnly       bool
	ExcludeLocked    bool
	MultiOnly        bool
	SingleOnly       bool
	PromoOnly        bool
	ExcludePromo     bool
	ExcludeAnimated  bool
	BoyGroups        bool
	GirlGroups       bool
	
	// Advanced user filters
	NewOnly      bool
	ExcludeNew   bool
	RatedOnly    bool
	ExcludeRated bool
	WishOnly     bool
	ExcludeWish  bool
	LastCard     bool
	Diff         int // 0=none, 1=diff, 2=miss
	
	// Amount filtering
	AmountMin   int64
	AmountMax   int64
	AmountExact int64
	
	// Star rating filtering (card property)
	StarMin   int
	StarMax   int
	StarExact int
	Stars     []int // Exact star matches
	AntiStars []int // Star exclusions
	
	// Experience filtering (user property)
	ExpMin   int64
	ExpMax   int64
	ExpExact int64
	
	// Sorting
	SortBy   string
	SortDesc bool
	
	// Query flags
	UserQuery bool // requires user-specific data
	EvalQuery bool // requires evaluation/rating data
	
	// Additional context
	TargetUserID string    // for diff queries
	LastDaily    time.Time // for "new" filter comparisons
}

// ConvertFromUtilsFilters converts utils.SearchFilters to repositories.SearchFilters
func ConvertFromUtilsFilters(utilsFilters interface{}) SearchFilters {
	// We need to handle this conversion carefully since utils.SearchFilters is in a different package
	// For now, we'll create a method to manually map the fields
	
	repoFilters := SearchFilters{}
	
	// This is a placeholder - the actual conversion would be done by the service layer
	// that has access to both packages
	
	return repoFilters
}

// ToUtilsFilters converts repositories.SearchFilters to a map that can be used by services
func (rf *SearchFilters) ToUtilsFilters() map[string]interface{} {
	return map[string]interface{}{
		"query":             rf.Query,
		"name":              rf.Name,
		"id":                rf.ID,
		"levels":            rf.Levels,
		"collections":       rf.Collections,
		"antiCollections":   rf.AntiCollections,
		"antiLevels":        rf.AntiLevels,
		"tags":              rf.Tags,
		"antiTags":          rf.AntiTags,
		"userID":            rf.UserID,
		"favorites":         rf.Favorites,
		"excludeFavorites":  rf.ExcludeFavorites,
		"lockedOnly":        rf.LockedOnly,
		"excludeLocked":     rf.ExcludeLocked,
		"multiOnly":         rf.MultiOnly,
		"singleOnly":        rf.SingleOnly,
		"promoOnly":         rf.PromoOnly,
		"excludePromo":      rf.ExcludePromo,
		"animated":          rf.Animated,
		"excludeAnimated":   rf.ExcludeAnimated,
		"boyGroups":         rf.BoyGroups,
		"girlGroups":        rf.GirlGroups,
		"newOnly":           rf.NewOnly,
		"excludeNew":        rf.ExcludeNew,
		"ratedOnly":         rf.RatedOnly,
		"excludeRated":      rf.ExcludeRated,
		"wishOnly":          rf.WishOnly,
		"excludeWish":       rf.ExcludeWish,
		"lastCard":          rf.LastCard,
		"diff":              rf.Diff,
		"amountMin":         rf.AmountMin,
		"amountMax":         rf.AmountMax,
		"amountExact":       rf.AmountExact,
		"starMin":           rf.StarMin,
		"starMax":           rf.StarMax,
		"starExact":         rf.StarExact,
		"stars":             rf.Stars,
		"antiStars":         rf.AntiStars,
		"expMin":            rf.ExpMin,
		"expMax":            rf.ExpMax,
		"expExact":          rf.ExpExact,
		"sortBy":            rf.SortBy,
		"sortDesc":          rf.SortDesc,
		"userQuery":         rf.UserQuery,
		"evalQuery":         rf.EvalQuery,
		"targetUserID":      rf.TargetUserID,
		"lastDaily":         rf.LastDaily,
	}
}

// IsUserQuery checks if this filter requires user-specific data
func (rf *SearchFilters) IsUserQuery() bool {
	return rf.UserQuery || 
		   rf.UserID != "" ||
		   rf.Favorites || rf.ExcludeFavorites ||
		   rf.LockedOnly || rf.ExcludeLocked ||
		   rf.MultiOnly || rf.SingleOnly ||
		   rf.NewOnly || rf.ExcludeNew ||
		   rf.RatedOnly || rf.ExcludeRated ||
		   rf.WishOnly || rf.ExcludeWish ||
		   rf.LastCard ||
		   rf.Diff > 0 ||
		   rf.AmountMin > 0 || rf.AmountMax > 0 || rf.AmountExact > 0 ||
		   rf.ExpMin > 0 || rf.ExpMax > 0 || rf.ExpExact > 0
}

// IsEvalQuery checks if this filter requires evaluation/rating data
func (rf *SearchFilters) IsEvalQuery() bool {
	return rf.EvalQuery || rf.RatedOnly || rf.ExcludeRated
}

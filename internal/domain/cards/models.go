package cards

import "time"

type Card struct {
	ID        int64
	Name      string
	Level     int
	Animated  bool
	Favorite  bool
	Amount    int64
	ColID     string
	Tags      []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

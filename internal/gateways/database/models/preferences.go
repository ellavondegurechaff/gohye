package models

// NotificationPreferences contains all notification settings
type NotificationPreferences struct {
	AucBidMe  bool `json:"aucbidme"`
	AucOutBid bool `json:"aucoutbid"`
	AucNewBid bool `json:"aucnewbid"`
	AucEnd    bool `json:"aucend"`
	Announce  bool `json:"announce"`
	Daily     bool `json:"daily"`
	Vote      bool `json:"vote"`
	Completed bool `json:"completed"`
	EffectEnd bool `json:"effectend"`
}

// InteractionPreferences contains all interaction settings
type InteractionPreferences struct {
	CanHas  bool `json:"canhas"`
	CanDiff bool `json:"candiff"`
	CanSell bool `json:"cansell"`
}

// ProfilePreferences contains all profile customization settings
type ProfilePreferences struct {
	Bio         string `json:"bio" validate:"max=500"`
	Title       string `json:"title" validate:"max=50"`
	Color       string `json:"color" validate:"hexcolor"`
	Card        string `json:"card"`
	FavComplete string `json:"favcomplete"`
	FavClout    string `json:"favclout"`
	Image       string `json:"image" validate:"url"`
	Reputation  int    `json:"reputation"`
}

// Preferences is the main preferences structure
type Preferences struct {
	Notifications NotificationPreferences `json:"notifications"`
	Interactions  InteractionPreferences  `json:"interactions"`
	Profile       ProfilePreferences      `json:"profile"`
}

// DefaultPreferences returns a new Preferences instance with default values
func DefaultPreferences() *Preferences {
	return &Preferences{
		Notifications: NotificationPreferences{
			AucBidMe:  true,
			AucOutBid: true,
			AucNewBid: true,
			AucEnd:    true,
			Announce:  true,
			Daily:     true,
			Vote:      true,
			Completed: true,
			EffectEnd: true,
		},
		Interactions: InteractionPreferences{
			CanHas:  true,
			CanDiff: true,
			CanSell: true,
		},
		Profile: ProfilePreferences{
			Color:      "#000000",
			Reputation: 0,
		},
	}
}

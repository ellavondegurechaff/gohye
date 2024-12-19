package models

type DeletionReport struct {
	CardID           int64 `json:"card_id"`
	UserCardsDeleted int   `json:"user_cards_deleted"`
	CardDeleted      bool  `json:"card_deleted"`
}

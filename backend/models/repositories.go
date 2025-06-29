package models

import (
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
)

// Repositories groups all repository interfaces for easy injection
type Repositories struct {
	User         repositories.UserRepository
	Card         repositories.CardRepository
	Collection   repositories.CollectionRepository
	UserCard     repositories.UserCardRepository
	Claim        repositories.ClaimRepository
	Auction      repositories.AuctionRepository
	Effect       repositories.EffectRepository
	Wishlist     repositories.WishlistRepository
	EconomyStats repositories.EconomyStatsRepository
}

// NewRepositories creates a new repositories group from individual repositories
func NewRepositories(
	user repositories.UserRepository,
	card repositories.CardRepository,
	collection repositories.CollectionRepository,
	userCard repositories.UserCardRepository,
	claim repositories.ClaimRepository,
	auction repositories.AuctionRepository,
	effect repositories.EffectRepository,
	wishlist repositories.WishlistRepository,
	economyStats repositories.EconomyStatsRepository,
) *Repositories {
	return &Repositories{
		User:         user,
		Card:         card,
		Collection:   collection,
		UserCard:     userCard,
		Claim:        claim,
		Auction:      auction,
		Effect:       effect,
		Wishlist:     wishlist,
		EconomyStats: economyStats,
	}
}
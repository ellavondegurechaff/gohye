package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/uptrace/bun"
)

type AuctionRepository interface {
	DB() *bun.DB
	Create(ctx context.Context, auction *models.Auction) error
	CreateWithTx(ctx context.Context, tx bun.Tx, auction *models.Auction) error
	GetByID(ctx context.Context, id int64) (*models.Auction, error)
	GetByAuctionID(ctx context.Context, auctionID string) (*models.Auction, error)
	GetActive(ctx context.Context) ([]*models.Auction, error)
	UpdateBid(ctx context.Context, auctionID int64, bidderID string, amount int64) error
	CompleteAuction(ctx context.Context, auctionID int64) error
	GetUserBids(ctx context.Context, userID string) ([]*models.AuctionBid, error)
	GetAuctionBids(ctx context.Context, auctionID int64) ([]*models.AuctionBid, error)
	CancelAuction(ctx context.Context, auctionID int64) error
	GetExpiredAuctions(ctx context.Context) ([]*models.Auction, error)
	UpdateAuctionMessage(ctx context.Context, auctionID int64, messageID string) error
	CompleteAuctionWithTransfer(ctx context.Context, auctionID int64) error
	GetActiveAuctionByCardAndSeller(ctx context.Context, cardID int64, sellerID string) (*models.Auction, error)
}

type auctionRepository struct {
	db *bun.DB
}

func NewAuctionRepository(db *bun.DB) AuctionRepository {
	return &auctionRepository{db: db}
}

func (r *auctionRepository) DB() *bun.DB {
	return r.db
}

func (r *auctionRepository) Create(ctx context.Context, auction *models.Auction) error {
	auction.CreatedAt = time.Now()
	auction.UpdatedAt = time.Now()
	auction.Status = models.AuctionStatusActive
	auction.BidCount = 0

	_, err := r.db.NewInsert().Model(auction).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create auction: %w", err)
	}
	return nil
}

func (r *auctionRepository) CreateWithTx(ctx context.Context, tx bun.Tx, auction *models.Auction) error {
	auction.CreatedAt = time.Now()
	auction.UpdatedAt = time.Now()
	auction.Status = models.AuctionStatusActive
	auction.BidCount = 0

	_, err := tx.NewInsert().Model(auction).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create auction: %w", err)
	}
	return nil
}

func (r *auctionRepository) GetByID(ctx context.Context, id int64) (*models.Auction, error) {
	auction := new(models.Auction)
	err := r.db.NewSelect().
		Model(auction).
		Where("id = ? AND status = ?", id, models.AuctionStatusActive).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get auction: %w", err)
	}
	return auction, nil
}

func (r *auctionRepository) GetActive(ctx context.Context) ([]*models.Auction, error) {
	var auctions []*models.Auction

	err := r.db.NewSelect().
		Model(&auctions).
		Where("status = ?", "active").
		Where("end_time > ?", time.Now()).
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get active auctions: %w", err)
	}

	return auctions, nil
}

func (r *auctionRepository) UpdateBid(ctx context.Context, auctionID int64, bidderID string, amount int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Update auction
	auction := new(models.Auction)
	err = tx.NewSelect().
		Model(auction).
		Where("id = ? AND status = ?", auctionID, models.AuctionStatusActive).
		For("UPDATE").
		Scan(ctx)

	if err != nil {
		return fmt.Errorf("failed to get auction for update: %w", err)
	}

	// Create bid record
	bid := &models.AuctionBid{
		AuctionID: auctionID,
		BidderID:  bidderID,
		Amount:    amount,
		Timestamp: time.Now(),
		CreatedAt: time.Now(),
	}

	_, err = tx.NewInsert().Model(bid).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create bid: %w", err)
	}

	// Update auction details
	_, err = tx.NewUpdate().
		Model(auction).
		Set("current_price = ?", amount).
		Set("top_bidder_id = ?", bidderID).
		Set("last_bid_time = ?", time.Now()).
		Set("bid_count = bid_count + 1").
		Set("updated_at = ?", time.Now()).
		Where("id = ?", auctionID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update auction: %w", err)
	}

	return tx.Commit()
}

func (r *auctionRepository) CompleteAuction(ctx context.Context, auctionID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Lock the auction record for update
	auction := new(models.Auction)
	err = tx.NewSelect().
		Model(auction).
		Where("id = ?", auctionID).
		For("UPDATE").
		Scan(ctx)

	if err != nil {
		return fmt.Errorf("failed to get auction for update: %w", err)
	}

	// Only update if the auction is still active
	if auction.Status != models.AuctionStatusActive {
		return fmt.Errorf("auction %d is not active (current status: %s)", auctionID, auction.Status)
	}

	// Update the auction status with retry logic
	result, err := tx.NewUpdate().
		Model((*models.Auction)(nil)).
		Set("status = ?", models.AuctionStatusCompleted).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", auctionID).
		Where("status = ?", models.AuctionStatusActive).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to complete auction: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("auction %d was not updated - may already be completed or cancelled", auctionID)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Info("Auction completed successfully",
		slog.Int64("auction_id", auctionID),
		slog.String("old_status", string(auction.Status)),
		slog.String("new_status", string(models.AuctionStatusCompleted)))

	return nil
}

func (r *auctionRepository) GetUserBids(ctx context.Context, userID string) ([]*models.AuctionBid, error) {
	var bids []*models.AuctionBid
	err := r.db.NewSelect().
		Model(&bids).
		Where("bidder_id = ?", userID).
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get user bids: %w", err)
	}
	return bids, nil
}

func (r *auctionRepository) GetAuctionBids(ctx context.Context, auctionID int64) ([]*models.AuctionBid, error) {
	var bids []*models.AuctionBid
	err := r.db.NewSelect().
		Model(&bids).
		Where("auction_id = ?", auctionID).
		Order("amount DESC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get auction bids: %w", err)
	}
	return bids, nil
}

func (r *auctionRepository) CancelAuction(ctx context.Context, auctionID int64) error {
	_, err := r.db.NewUpdate().
		Model((*models.Auction)(nil)).
		Set("status = ?", models.AuctionStatusCancelled).
		Set("updated_at = ?", time.Now()).
		Where("id = ? AND status = ?",
			auctionID,
			models.AuctionStatusActive,
		).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to cancel auction: %w", err)
	}
	return nil
}

func (r *auctionRepository) GetExpiredAuctions(ctx context.Context) ([]*models.Auction, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	var auctions []*models.Auction
	err = tx.NewSelect().
		Model(&auctions).
		Where("status = ?", models.AuctionStatusActive).
		Where("end_time <= ?", time.Now()).
		For("UPDATE SKIP LOCKED"). // Skip locked records to prevent conflicts
		Order("end_time ASC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get expired auctions: %w", err)
	}

	if len(auctions) > 0 {
		// Bulk update all found auctions to completed status
		ids := make([]int64, len(auctions))
		for i, auction := range auctions {
			ids[i] = auction.ID
		}

		_, err = tx.NewUpdate().
			Model((*models.Auction)(nil)).
			Set("status = ?", models.AuctionStatusCompleted).
			Set("updated_at = ?", time.Now()).
			Where("id IN (?)", bun.In(ids)).
			Where("status = ?", models.AuctionStatusActive).
			Where("end_time <= ?", time.Now()).
			Exec(ctx)

		if err != nil {
			return nil, fmt.Errorf("failed to bulk update expired auctions: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return auctions, nil
}

func (r *auctionRepository) UpdateAuctionMessage(ctx context.Context, auctionID int64, messageID string) error {
	_, err := r.db.NewUpdate().
		Model((*models.Auction)(nil)).
		Set("message_id = ?", messageID).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", auctionID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update auction message: %w", err)
	}
	return nil
}

func (r *auctionRepository) GetByAuctionID(ctx context.Context, auctionID string) (*models.Auction, error) {
	auction := new(models.Auction)
	err := r.db.NewSelect().
		Model(auction).
		Where("auction_id = ?", auctionID).
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("auction not found")
		}
		return nil, fmt.Errorf("failed to get auction: %w", err)
	}

	return auction, nil
}

func (r *auctionRepository) CompleteAuctionWithTransfer(ctx context.Context, auctionID int64) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Get auction details first
	auction := new(models.Auction)
	err = tx.NewSelect().
		Model(auction).
		Where("id = ?", auctionID).
		Scan(ctx)

	if err != nil {
		return fmt.Errorf("failed to get auction: %w", err)
	}

	if auction.TopBidderID == "" {
		// No bids - return card to seller
		result, err := tx.NewUpdate().
			Model(&models.UserCard{}).
			Set("amount = amount + 1").
			Set("updated_at = ?", time.Now()).
			Where("user_id = ? AND card_id = ?", auction.SellerID, auction.CardID).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to return card to seller: %w", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}

		if rowsAffected == 0 {
			// If no rows affected, create new entry
			_, err = tx.NewInsert().
				Model(&models.UserCard{
					UserID:    auction.SellerID,
					CardID:    auction.CardID,
					Amount:    1,
					Obtained:  time.Now(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}).
				Exec(ctx)

			if err != nil {
				return fmt.Errorf("failed to create new card entry for seller: %w", err)
			}
		}

		slog.Info("Card returned to seller - no bids",
			slog.String("seller_id", auction.SellerID),
			slog.Int64("card_id", auction.CardID))
	} else {
		// Has winner - transfer card to winner
		// Remove card from seller
		result, err := tx.NewUpdate().
			Model((*models.UserCard)(nil)).
			Set("amount = amount - 1").
			Where("user_id = ? AND card_id = ? AND amount > 0", auction.SellerID, auction.CardID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to update seller's card: %w", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil || rowsAffected == 0 {
			return fmt.Errorf("failed to update seller's card amount")
		}

		// Cleanup zero amount cards
		cleanupResult, err := tx.NewDelete().
			Model((*models.UserCard)(nil)).
			Where("user_id = ? AND card_id = ? AND amount <= 0", auction.SellerID, auction.CardID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to cleanup zero amount card: %w", err)
		}

		if cleaned, _ := cleanupResult.RowsAffected(); cleaned > 0 {
			slog.Info("Cleaned up zero amount card",
				slog.String("seller_id", auction.SellerID),
				slog.Int64("card_id", auction.CardID))
		}

		// Add card to winner
		_, err = tx.NewInsert().
			Model(&models.UserCard{
				UserID:    auction.TopBidderID,
				CardID:    auction.CardID,
				Amount:    1,
				Obtained:  time.Now(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}).
			On("CONFLICT (user_id, card_id) DO UPDATE").
			Set("amount = user_cards.amount + 1").
			Set("updated_at = ?", time.Now()).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to transfer card to winner: %w", err)
		}
	}

	// Update auction status
	_, err = tx.NewUpdate().
		Model(auction).
		Set("status = ?", models.AuctionStatusCompleted).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", auctionID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update auction status: %w", err)
	}

	return tx.Commit()
}

func (r *auctionRepository) GetActiveAuctionByCardAndSeller(ctx context.Context, cardID int64, sellerID string) (*models.Auction, error) {
	auction := new(models.Auction)
	err := r.db.NewSelect().
		Model(auction).
		Where("card_id = ? AND seller_id = ? AND status = ?", cardID, sellerID, models.AuctionStatusActive).
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active auction: %w", err)
	}

	return auction, nil
}

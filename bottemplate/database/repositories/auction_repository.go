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
	CompleteAuctionWithTransferAndGet(ctx context.Context, auctionID int64) (*models.Auction, error)
	handleWinningBidTransfer(ctx context.Context, tx bun.Tx, auction *models.Auction) error
	GetRecentCompletedAuctions(ctx context.Context, cardID int64, limit int) ([]*models.Auction, error)
	AuctionIDExists(ctx context.Context, auctionID string) (bool, error)
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
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
	})
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
		Limit(10). // Process in smaller batches
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get expired auctions: %w", err)
	}

	if len(auctions) > 0 {
		// Mark these auctions as being processed
		ids := make([]int64, len(auctions))
		for i, auction := range auctions {
			ids[i] = auction.ID
		}

		_, err = tx.NewUpdate().
			Model((*models.Auction)(nil)).
			Set("updated_at = ?", time.Now()).
			Where("id IN (?)", bun.In(ids)).
			Where("status = ?", models.AuctionStatusActive).
			Where("end_time <= ?", time.Now()).
			Exec(ctx)

		if err != nil {
			return nil, fmt.Errorf("failed to mark auctions as processing: %w", err)
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

	// Lock and get auction details
	auction := new(models.Auction)
	err = tx.NewSelect().
		Model(auction).
		Where("id = ? AND status = ?", auctionID, models.AuctionStatusActive).
		For("UPDATE").
		Scan(ctx)

	if err != nil {
		return fmt.Errorf("failed to get auction: %w", err)
	}

	// If auction is already completed, skip
	if auction.Status != models.AuctionStatusActive {
		return nil
	}

	if auction.TopBidderID == "" {
		// No bids - return card to seller
		result, err := tx.NewUpdate().
			Model((*models.UserCard)(nil)).
			Set("amount = amount + 1").
			Set("updated_at = ?", time.Now()).
			Where("user_id = ? AND card_id = ?", auction.SellerID, auction.CardID).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to return card to seller: %w", err)
		}

		// If seller doesn't have the card yet, create new entry
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}

		if affected == 0 {
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
	} else {
		// Transfer to winner - first try update
		result, err := tx.NewUpdate().
			Model((*models.UserCard)(nil)).
			Set("amount = amount + 1").
			Set("updated_at = ?", time.Now()).
			Where("user_id = ? AND card_id = ?", auction.TopBidderID, auction.CardID).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to update winner's card: %w", err)
		}

		// If winner doesn't have the card yet, create new entry
		if affected, _ := result.RowsAffected(); affected == 0 {
			_, err = tx.NewInsert().
				Model(&models.UserCard{
					UserID:    auction.TopBidderID,
					CardID:    auction.CardID,
					Amount:    1,
					Obtained:  time.Now(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}).
				Exec(ctx)

			if err != nil {
				return fmt.Errorf("failed to create new card entry for winner: %w", err)
			}
		}

		// Transfer currency to seller
		_, err = tx.NewUpdate().
			Model((*models.User)(nil)).
			Set("balance = balance + ?", auction.CurrentPrice).
			Where("id = ?", auction.SellerID).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to transfer balance to seller: %w", err)
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

func (r *auctionRepository) CompleteAuctionWithTransferAndGet(ctx context.Context, auctionID int64) (*models.Auction, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Lock and get auction details
	auction := new(models.Auction)
	err = tx.NewSelect().
		Model(auction).
		Where("id = ? AND status = ?", auctionID, models.AuctionStatusActive).
		For("UPDATE").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get auction: %w", err)
	}

	// If auction is already completed, return current state
	if auction.Status != models.AuctionStatusActive {
		return auction, nil
	}

	if auction.TopBidderID == "" {
		// Handle no bids case
		result, err := tx.NewUpdate().
			Model((*models.UserCard)(nil)).
			Set("amount = amount + 1").
			Set("updated_at = ?", time.Now()).
			Where("user_id = ? AND card_id = ?", auction.SellerID, auction.CardID).
			Exec(ctx)

		if err != nil {
			return nil, fmt.Errorf("failed to return card to seller: %w", err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("failed to get rows affected: %w", err)
		}

		if affected == 0 {
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
				return nil, fmt.Errorf("failed to create new card entry for seller: %w", err)
			}
		}
	} else {
		// Handle winning bid case
		if err := r.handleWinningBidTransfer(ctx, tx, auction); err != nil {
			return nil, err
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
		return nil, fmt.Errorf("failed to update auction status: %w", err)
	}

	// Get final state within transaction
	updatedAuction := new(models.Auction)
	err = tx.NewSelect().
		Model(updatedAuction).
		Where("id = ?", auctionID).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get final auction state: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return updatedAuction, nil
}

func (r *auctionRepository) handleWinningBidTransfer(ctx context.Context, tx bun.Tx, auction *models.Auction) error {
	// Transfer to winner - first try update
	result, err := tx.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("amount = amount + 1").
		Set("updated_at = ?", time.Now()).
		Where("user_id = ? AND card_id = ?", auction.TopBidderID, auction.CardID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update winner's card: %w", err)
	}

	// If winner doesn't have the card yet, create new entry
	if affected, _ := result.RowsAffected(); affected == 0 {
		_, err = tx.NewInsert().
			Model(&models.UserCard{
				UserID:    auction.TopBidderID,
				CardID:    auction.CardID,
				Amount:    1,
				Obtained:  time.Now(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to create new card entry for winner: %w", err)
		}
	}

	// Transfer currency to seller - Fixed the WHERE clause to use discord_id
	_, err = tx.NewUpdate().
		Model((*models.User)(nil)).
		Set("balance = balance + ?", auction.CurrentPrice).
		Where("discord_id = ?", auction.SellerID). // Changed from id to discord_id
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to transfer balance to seller: %w", err)
	}

	// Verify the balance transfer
	var seller models.User
	err = tx.NewSelect().
		Model(&seller).
		Where("discord_id = ?", auction.SellerID).
		Scan(ctx)

	if err != nil {
		return fmt.Errorf("failed to verify seller balance transfer: %w", err)
	}

	slog.Info("Successfully transferred auction proceeds",
		slog.String("seller_id", auction.SellerID),
		slog.Int64("amount", auction.CurrentPrice),
		slog.Int64("new_balance", seller.Balance))

	return nil
}

func (r *auctionRepository) GetRecentCompletedAuctions(ctx context.Context, cardID int64, limit int) ([]*models.Auction, error) {
	var auctions []*models.Auction
	err := r.db.NewSelect().
		Model(&auctions).
		Where("card_id = ?", cardID).
		Where("status = ?", models.AuctionStatusCompleted).
		Where("top_bidder_id IS NOT NULL"). // Only include successful auctions
		OrderExpr("end_time DESC").
		Limit(limit).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get recent completed auctions: %w", err)
	}

	return auctions, nil
}

func (r *auctionRepository) AuctionIDExists(ctx context.Context, auctionID string) (bool, error) {
	exists, err := r.db.NewSelect().
		Model((*models.Auction)(nil)).
		Where("auction_id = ?", auctionID).
		Exists(ctx)

	return exists, err
}

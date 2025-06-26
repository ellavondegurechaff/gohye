package utils

// EXAMPLE: How to use the new economic utilities
// This file demonstrates the usage patterns - it should not be included in production

/*
// OLD PATTERN (from pricing.go):
func (pc *PriceCalculator) calculateBasePrice(card models.Card) int64 {
	basePrice := InitialBasePrice * int64(math.Pow(LevelMultiplier, float64(card.Level-1)))
	return max(basePrice, MinPrice)
}

func (pc *PriceCalculator) applyPriceLimits(price int64) int64 {
	if price < MinPrice {
		return MinPrice
	}
	if price > MaxPrice {
		return MaxPrice
	}
	return price
}

// NEW PATTERN (using utilities):
func (pc *PriceCalculator) calculateBasePrice(card models.Card) int64 {
	cv := NewCardValidation()
	return cv.CalculateBasePrice(card)
}

func (pc *PriceCalculator) applyPriceLimits(price int64) int64 {
	cv := NewCardValidation()
	return cv.ApplyPriceLimits(price)
}

// OLD PATTERN (from forge.go):
func (fm *ForgeManager) CalculateForgeCost(ctx context.Context, card1, card2 *models.Card) (int64, error) {
	// ... get prices ...
	avgPrice := (price1 + price2) / 2
	forgeCost := int64(math.Max(float64(avgPrice)*ForgeCostMultiplier, float64(MinForgeCost)))
	return forgeCost, nil
}

// NEW PATTERN (using utilities):
func (fm *ForgeManager) CalculateForgeCost(ctx context.Context, card1, card2 *models.Card) (int64, error) {
	// ... get prices ...
	cv := NewCardValidation()
	forgeCost := cv.CalculateForgeCost(price1, price2)
	return forgeCost, nil
}

// OLD PATTERN (from vials.go):
func (vm *VialManager) CalculateVialYield(ctx context.Context, card *models.Card) (int64, error) {
	// ... get price ...
	var vialRate float64
	switch card.Level {
	case 1:
		vialRate = VialRate1Star
	case 2:
		vialRate = VialRate2Star
	case 3:
		vialRate = VialRate3Star
	default:
		return 0, fmt.Errorf("invalid card level for liquefying: %d", card.Level)
	}
	vials := int64(math.Floor(float64(price) * vialRate))
	return vials, nil
}

// NEW PATTERN (using utilities):
func (vm *VialManager) CalculateVialYield(ctx context.Context, card *models.Card) (int64, error) {
	// ... get price ...
	cv := NewCardValidation()
	return cv.CalculateVialYield(card, price)
}

// OLD PATTERN (from auction_manager.go):
tx, err := m.repo.DB().BeginTx(ctx, &sql.TxOptions{
	Isolation: sql.LevelSerializable,
})
if err != nil {
	return fmt.Errorf("failed to start transaction: %w", err)
}
defer tx.Rollback()

// ... operations ...

if err := tx.Commit(); err != nil {
	return fmt.Errorf("failed to commit transaction: %w", err)
}

// NEW PATTERN (using utilities):
tm := NewTransactionManager(m.repo.DB())
return tm.WithTransaction(ctx, func(ctx context.Context, tx bun.Tx) error {
	// ... operations ...
	return nil
})

// OLD PATTERN (from auction_manager.go):
_, err = tx.NewUpdate().
	Model((*models.User)(nil)).
	Set("balance = balance - ?", amount).
	Where("discord_id = ?", bidderID).
	Exec(ctx)

// NEW PATTERN (using utilities):
tm := NewTransactionManager(db)
err := tm.UpdateUserBalance(ctx, tx, bidderID, -amount)
*/
package cardleveling

import "time"

type Config struct {
	// Base EXP requirements per level
	BaseExpRequirements map[int]int64

	// EXP gain multipliers
	ExpMultipliers struct {
		Level1 float64
		Level2 float64
		Level3 float64
		Level4 float64
	}

	// Cooldowns and limits
	ExpGainCooldown    time.Duration
	DailyExpGainLimit  int
	WeeklyExpGainLimit int

	// Bonus configurations
	CriticalExpChance float64
	CriticalExpBonus  float64
	ComboBonus        float64
}

func NewDefaultConfig() *Config {
	return &Config{
		BaseExpRequirements: map[int]int64{
			1: 4500,   // Level 1 -> 2 (4.5x harder)
			2: 15000,  // Level 2 -> 3 (5x harder)
			3: 45000,  // Level 3 -> 4 (4.5x harder)
			4: 150000, // Level 4 -> 5 (5x harder)
		},
		ExpMultipliers: struct {
			Level1 float64
			Level2 float64
			Level3 float64
			Level4 float64
		}{
			Level1: 0.8, // Reduced from 1.0
			Level2: 0.6, // Reduced from 0.8
			Level3: 0.4, // Reduced from 0.6
			Level4: 0.2, // Reduced from 0.4
		},
		ExpGainCooldown:    0,    // Temporarily disabled cooldown
		DailyExpGainLimit:  30,   // Reduced from 50
		WeeklyExpGainLimit: 180,  // Reduced from 300
		CriticalExpChance:  0.10, // Reduced from 0.15
		CriticalExpBonus:   1.5,  // Reduced from 2.0
		ComboBonus:         0.05, // Reduced from 0.1
	}
}

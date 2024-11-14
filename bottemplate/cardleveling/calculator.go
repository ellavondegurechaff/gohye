package cardleveling

import (
	"math"
	"math/rand"
)

type Calculator struct {
	config *Config
}

func NewCalculator(config *Config) *Calculator {
	return &Calculator{config: config}
}

func (c *Calculator) CalculateExpRequirement(level int) int64 {
	baseExp := c.config.BaseExpRequirements[level]
	return int64(float64(baseExp) * math.Pow(1.5, float64(level-1)))
}

func (c *Calculator) CalculateExpGain(level int, stats *CardLevelingStats) *ExpGainConfig {
	// Base gain calculation
	baseGain := int64(100 * math.Pow(0.8, float64(level-1)))

	// Level penalty
	levelPenalty := c.config.ExpMultipliers.Level1
	switch level {
	case 2:
		levelPenalty = c.config.ExpMultipliers.Level2
	case 3:
		levelPenalty = c.config.ExpMultipliers.Level3
	case 4:
		levelPenalty = c.config.ExpMultipliers.Level4
	}

	// Activity bonus (more active = better gains)
	activityBonus := 1.0
	if stats.WeeklyExpGains > 0 {
		activityBonus += float64(stats.WeeklyExpGains) * 0.01
	}

	// Time bonus (less gains today = better gains)
	timeBonus := 1.0
	if stats.DailyExpGains < c.config.DailyExpGainLimit/2 {
		timeBonus = 1.2
	}

	return &ExpGainConfig{
		BaseGain:      baseGain,
		LevelPenalty:  levelPenalty,
		ActivityBonus: activityBonus,
		TimeBonus:     timeBonus,
	}
}

func (c *Calculator) CalculateFinalExp(config *ExpGainConfig) int64 {
	baseExp := float64(config.BaseGain)

	// Apply all multipliers
	finalExp := baseExp *
		config.LevelPenalty *
		config.ActivityBonus *
		config.TimeBonus

	// Critical exp chance
	if rand.Float64() < c.config.CriticalExpChance {
		finalExp *= c.config.CriticalExpBonus
	}

	return int64(math.Max(1, finalExp))
}

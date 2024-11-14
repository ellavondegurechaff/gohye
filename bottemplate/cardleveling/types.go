package cardleveling

import "time"

type LevelingResult struct {
	Success      bool
	NewLevel     int
	CurrentExp   int64
	RequiredExp  int64
	ExpGained    int64
	Bonuses      []string
	CombinedCard bool
}

type ExpGainConfig struct {
	BaseGain      int64
	LevelPenalty  float64
	ActivityBonus float64
	TimeBonus     float64
}

type CardLevelingStats struct {
	LastExpGain    time.Time
	DailyExpGains  int
	WeeklyExpGains int
	TotalExpGains  int64
}

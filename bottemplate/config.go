package bottemplate

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/disgoorg/snowflake/v2"
	"github.com/pelletier/go-toml/v2"
)

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config: %w", err)
	}

	var cfg Config
	if err = toml.NewDecoder(file).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

type Config struct {
	Log    LogConfig `toml:"log"`
	Bot    BotConfig `toml:"bot"`
	DB     DBConfig  `toml:"db"`
	Spaces struct {
		Key      string `toml:"key"`
		Secret   string `toml:"secret"`
		Region   string `toml:"region"`
		Bucket   string `toml:"bucket"`
		CardRoot string `toml:"cardroot"` // Add this field
	} `toml:"spaces"`
}

type BotConfig struct {
	DevGuilds []snowflake.ID `toml:"dev_guilds"`
	Token     string         `toml:"token"`
}

type LogConfig struct {
	Level     slog.Level `toml:"level"`
	Format    string     `toml:"format"`
	AddSource bool       `toml:"add_source"`
}

type DBConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	Database string `toml:"database"`
	PoolSize int    `toml:"pool_size"`
}

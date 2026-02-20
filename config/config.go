package config

import (
	"errors"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

// Config holds application configuration.
type Config struct {
	DiscordToken string
	GuildID      string
	DatabasePath string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		slog.Warn("no .env file found, using environment variables directly")
	}

	cfg := &Config{
		DiscordToken: os.Getenv("DISCORD_TOKEN"),
		GuildID:      os.Getenv("GUILD_ID"),
		DatabasePath: os.Getenv("DATABASE_PATH"),
	}

	if cfg.DatabasePath == "" {
		cfg.DatabasePath = "async_traitors.db"
	}

	if cfg.DiscordToken == "" {
		return nil, errors.New("DISCORD_TOKEN is required")
	}

	return cfg, nil
}

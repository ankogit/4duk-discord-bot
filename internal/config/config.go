package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the bot
type Config struct {
	DiscordToken          string
	RadioURL              string
	MaxReconnectAttempts  int
	ReconnectBackoffBase  time.Duration
	VoiceCheckInterval    time.Duration
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Try to load .env file (optional)
	_ = godotenv.Load()

	discordToken := os.Getenv("DISCORD_TOKEN")
	if discordToken == "" {
		return nil, fmt.Errorf("DISCORD_TOKEN not set in environment")
	}

	radioURL := os.Getenv("RADIO_URL")
	if radioURL == "" {
		radioURL = "http://radio.4duk.ru/4duk128.mp3"
	}

	return &Config{
		DiscordToken:         discordToken,
		RadioURL:             radioURL,
		MaxReconnectAttempts:  5,
		ReconnectBackoffBase:  2 * time.Second,
		VoiceCheckInterval:   20 * time.Second,
	}, nil
}


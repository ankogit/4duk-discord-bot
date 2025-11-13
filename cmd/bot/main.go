package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/ankogit/4duk-discord-bot/internal/bot"
	"github.com/ankogit/4duk-discord-bot/internal/config"
)

func main() {
	// Setup logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logger.SetLevel(logrus.InfoLevel)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.WithError(err).Fatal("Failed to load configuration")
	}

	// Create bot
	discordBot, err := bot.New(cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create bot")
	}

	// Start bot
	err = discordBot.Start()
	if err != nil {
		logger.WithError(err).Fatal("Failed to start bot")
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Stop bot gracefully
	err = discordBot.Stop()
	if err != nil {
		logger.WithError(err).Error("Error stopping bot")
		os.Exit(1)
	}

	logger.Info("Bot stopped successfully")
}


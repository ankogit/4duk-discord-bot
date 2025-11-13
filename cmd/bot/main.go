package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

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

	// Run bot with automatic restart on panic
	// This handles panics from discordgo fork
	runBotWithRecovery(cfg, logger)
}

func runBotWithRecovery(cfg *config.Config, logger *logrus.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		botChan := make(chan error, 1)
		var discordBot *bot.Bot

		// Run bot in goroutine to catch panics
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.WithField("panic", r).
						WithField("stack", string(debug.Stack())).
						Error("CRITICAL: Panic caught (bug in discordgo fork) - restarting bot")
					
					// Clean up bot if it exists
					if discordBot != nil {
						func() {
							defer func() {
								if r := recover(); r != nil {
									logger.WithField("panic", r).Error("Panic during bot cleanup, ignoring")
								}
							}()
							_ = discordBot.Stop()
						}()
					}
					
					botChan <- fmt.Errorf("panic: %v", r)
				}
			}()

			// Create bot
			var err error
			discordBot, err = bot.New(cfg, logger)
			if err != nil {
				logger.WithError(err).Fatal("Failed to create bot")
			}

			// Start bot
			err = discordBot.Start()
			if err != nil {
				logger.WithError(err).Fatal("Failed to start bot")
			}

			botChan <- nil

			// Wait for interrupt
			<-sigChan
			
			// Stop gracefully
			err = discordBot.Stop()
			if err != nil {
				logger.WithError(err).Error("Error stopping bot")
			} else {
				logger.Info("Bot stopped successfully")
			}
			os.Exit(0)
		}()

		// Wait for bot to start or panic
		err := <-botChan
		if err != nil {
			logger.Warnf("Bot crashed, waiting 5 seconds before restart: %v", err)
			time.Sleep(5 * time.Second)
			continue // Restart bot
		}

		// If we get here, bot started successfully
		// Wait for interrupt signal
		<-sigChan
		return
	}
}


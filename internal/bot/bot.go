package bot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"

	"github.com/ankogit/4duk-discord-bot/internal/audio"
	"github.com/ankogit/4duk-discord-bot/internal/config"
	"github.com/ankogit/4duk-discord-bot/internal/radio"
)

// Bot represents the Discord bot
type Bot struct {
	session      *discordgo.Session
	config       *config.Config
	radioManager *radio.Manager
	streamer     *audio.Streamer
	encoderPool  *audio.EncoderPool
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	logger       *logrus.Logger
}

// New creates a new bot instance
func New(cfg *config.Config, logger *logrus.Logger) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	// Set intents
	session.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsGuildVoiceStates

	ctx, cancel := context.WithCancel(context.Background())

	encoderPool := audio.NewEncoderPool()
	radioManager := radio.NewManager()
	streamer := audio.NewStreamer(cfg.RadioURL, encoderPool, logger)

	bot := &Bot{
		session:      session,
		config:       cfg,
		radioManager: radioManager,
		streamer:     streamer,
		encoderPool:  encoderPool,
		ctx:          ctx,
		cancel:       cancel,
		logger:       logger,
	}

	// Register event handlers
	session.AddHandler(bot.onReady)
	session.AddHandler(bot.onMessageCreate)
	session.AddHandler(bot.onVoiceStateUpdate)

	return bot, nil
}

// Start starts the bot
func (b *Bot) Start() error {
	err := b.session.Open()
	if err != nil {
		return fmt.Errorf("failed to open session: %w", err)
	}

	b.logger.Info("Bot started successfully")
	return nil
}

// Stop stops the bot gracefully
func (b *Bot) Stop() error {
	b.logger.Info("Shutting down...")

	// Cancel context to stop all goroutines
	b.cancel()

	// Disconnect all voice connections
	guildIDs := b.radioManager.GetAllGuildIDs()
	for _, guildID := range guildIDs {
		state := b.radioManager.GetOrCreate(guildID)
		state.SetActive(false)

		if vc, exists := b.session.VoiceConnections[guildID]; exists {
			// Remove from map first to prevent Kill() panic
			delete(b.session.VoiceConnections, guildID)
			func() {
				defer func() {
					if r := recover(); r != nil {
						b.logger.Debugf("[%s] Panic during disconnect (ignored): %v", guildID, r)
					}
				}()
				_ = vc.Disconnect(context.Background())
			}()
		}

		// Cleanup encoder
		b.encoderPool.Remove(guildID)
	}

	// Close Discord session
	err := b.session.Close()
	if err != nil {
		b.logger.WithError(err).Error("Error closing Discord session")
	}

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		b.logger.Info("All goroutines finished")
	case <-time.After(10 * time.Second):
		b.logger.Warn("Timeout waiting for goroutines to finish")
	}

	return nil
}


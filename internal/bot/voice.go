package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

// connectToChannel connects to a voice channel
func (b *Bot) connectToChannel(s *discordgo.Session, guildID, channelID string) (*discordgo.VoiceConnection, error) {
	// Check if already connected and ready
	if vc, exists := s.VoiceConnections[guildID]; exists {
		if vc.Status == discordgo.VoiceConnectionStatusReady {
			// Check if we're in the right channel by checking voice state
			vs, err := s.State.VoiceState(guildID, s.State.User.ID)
			if err == nil && vs != nil && vs.ChannelID == channelID {
				return vc, nil
			}
		}
		// Disconnect from current channel if wrong channel or not ready
		// Remove from map first to prevent Kill() panic
		delete(s.VoiceConnections, guildID)
		func() {
			defer func() {
				if r := recover(); r != nil {
					b.logger.Debugf("[%s] Panic during disconnect (ignored): %v", guildID, r)
				}
			}()
			_ = vc.Disconnect(context.Background())
		}()
	}

	// Connect to the channel with retry logic
	// mute=false, deaf=true (bot should not hear other users)
	var vc *discordgo.VoiceConnection
	var err error

	// Create context with timeout for connection
	ctx, cancel := context.WithTimeout(b.ctx, 15*time.Second)
	defer cancel()

	// Try connecting up to 3 times with delays
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Wait before retry
			time.Sleep(time.Duration(attempt) * time.Second)
			// Disconnect any existing connection
			if existing, exists := s.VoiceConnections[guildID]; exists {
				// Remove from map first to prevent Kill() panic
				delete(s.VoiceConnections, guildID)
				// Try to disconnect, but ignore errors/panics
				func() {
					defer func() {
						if r := recover(); r != nil {
							b.logger.Debugf("[%s] Panic during disconnect (ignored): %v", guildID, r)
						}
					}()
					_ = existing.Disconnect(context.Background())
				}()
				time.Sleep(500 * time.Millisecond)
			}
		}

		// Wrap ChannelVoiceJoin in recover to catch panics from fork
		func() {
			defer func() {
				if r := recover(); r != nil {
					b.logger.Warnf("[%s] Panic during ChannelVoiceJoin: %v", guildID, r)
					// Remove any bad connection from map
					if badVC, exists := s.VoiceConnections[guildID]; exists {
						delete(s.VoiceConnections, guildID)
						// Try to clean up, but ignore panics
						func() {
							defer func() { recover() }()
							_ = badVC.Disconnect(context.Background())
						}()
					}
					err = fmt.Errorf("panic during join: %v", r)
				}
			}()
			vc, err = s.ChannelVoiceJoin(ctx, guildID, channelID, false, true)
		}()

		if err == nil && vc != nil {
			break
		}

		b.logger.Warnf("[%s] Voice join attempt %d failed: %v", guildID, attempt+1, err)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to join voice channel after 3 attempts: %w", err)
	}

	// Wait for voice connection to be ready
	// Check Ready status periodically
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if vc.Status == discordgo.VoiceConnectionStatusReady {
				// Wait a bit more for connection to fully stabilize
				// This ensures the websocket is fully established
				time.Sleep(500 * time.Millisecond)
				// Double-check connection is still ready
				if vc.Status != discordgo.VoiceConnectionStatusReady {
					continue
				}
				b.logger.Infof("[%s] Connected to voice channel %s", guildID, channelID)
				return vc, nil
			}
		case <-timeout.C:
			// Remove from map first to prevent Kill() panic
			delete(s.VoiceConnections, guildID)
			func() {
				defer func() { recover() }()
				_ = vc.Disconnect(context.Background())
			}()
			return nil, fmt.Errorf("timeout waiting for voice connection")
		case <-b.ctx.Done():
			// Remove from map first to prevent Kill() panic
			delete(s.VoiceConnections, guildID)
			func() {
				defer func() { recover() }()
				_ = vc.Disconnect(b.ctx)
			}()
			return nil, b.ctx.Err()
		}
	}
}

// startRadio starts playing radio on a voice connection
func (b *Bot) startRadio(vc *discordgo.VoiceConnection, guildID string) error {
	if vc == nil || vc.Status != discordgo.VoiceConnectionStatusReady {
		return fmt.Errorf("voice connection not ready")
	}

	state := b.radioManager.GetOrCreate(guildID)

	// Create context for this stream
	streamCtx, cancel := context.WithCancel(b.ctx)

	// Start playing in a goroutine
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer cancel()

		// Check if still active periodically
		isActive := func() bool {
			return state.IsActive()
		}

		err := b.streamer.Stream(streamCtx, vc, guildID, isActive)
		if err != nil {
			b.logger.WithError(err).Warnf("[%s] Stream ended", guildID)
		}

		// Trigger reconnect if still active
		if state.IsActive() {
			b.wg.Add(1)
			go func() {
				defer b.wg.Done()
				b.reconnectRadio(guildID)
			}()
		}
	}()

	return nil
}

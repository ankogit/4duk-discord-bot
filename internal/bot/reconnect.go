package bot

import (
	"time"

	"github.com/bwmarrin/discordgo"
)

// reconnectRadio attempts to reconnect the radio
func (b *Bot) reconnectRadio(guildID string) {
	state, exists := b.radioManager.Get(guildID)
	if !exists {
		return
	}

	if !state.IsActive() {
		b.logger.Infof("[%s] Radio not active anymore, skipping reconnect", guildID)
		return
	}

	attempts := state.GetReconnectAttempts()
	channelID := state.GetChannelID()

	if attempts >= b.config.MaxReconnectAttempts {
		b.logger.Errorf("[%s] Reached max reconnect attempts (%d). Giving up", guildID, attempts)
		return
	}

	if channelID == "" {
		b.logger.Warnf("[%s] No channel recorded to reconnect", guildID)
		return
	}

	// Calculate backoff
	backoff := time.Duration(1<<uint(attempts)) * b.config.ReconnectBackoffBase
	b.logger.Infof("[%s] Reconnect attempt #%d, sleeping %v before trying", guildID, attempts+1, backoff)

	select {
	case <-time.After(backoff):
	case <-b.ctx.Done():
		return
	}

	state.IncrementReconnectAttempts()

	// Connect to channel
	vc, err := b.connectToChannel(b.session, guildID, channelID)
	if err != nil {
		b.logger.WithError(err).Errorf("[%s] Failed to reconnect to channel", guildID)
		// Schedule another attempt
		if state.IsActive() && state.GetReconnectAttempts() < b.config.MaxReconnectAttempts {
			b.wg.Add(1)
			go func() {
				defer b.wg.Done()
				b.reconnectRadio(guildID)
			}()
		}
		return
	}

	// Reset attempts on success
	state.ResetReconnectAttempts()

	// Start radio again
	err = b.startRadio(vc, guildID)
	if err != nil {
		b.logger.WithError(err).Errorf("[%s] Failed to restart radio after reconnect", guildID)
	}
}

// voiceCheckLoop periodically checks voice connections
func (b *Bot) voiceCheckLoop() {
	defer b.wg.Done()

	ticker := time.NewTicker(b.config.VoiceCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			b.checkVoiceConnections()
		}
	}
}

// checkVoiceConnections checks all active voice connections
func (b *Bot) checkVoiceConnections() {
	guildIDs := b.radioManager.GetAllGuildIDs()

	for _, guildID := range guildIDs {
		state, exists := b.radioManager.Get(guildID)
		if !exists || !state.IsActive() {
			continue
		}

		vc, exists := b.session.VoiceConnections[guildID]
		if !exists || vc == nil || vc.Status != discordgo.VoiceConnectionStatusReady {
			b.logger.Infof("[%s] voice_check_loop: detected dead vc -> scheduling reconnect", guildID)
			b.wg.Add(1)
			go func(gid string) {
				defer b.wg.Done()
				b.reconnectRadio(gid)
			}(guildID)
		}
	}
}


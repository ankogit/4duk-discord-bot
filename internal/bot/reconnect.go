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

	// Check if there are users in the channel before reconnecting
	userCount := b.countUsersInChannel(guildID, channelID)
	if userCount == 0 {
		b.logger.Infof("[%s] No users in channel %s, stopping radio instead of reconnecting", guildID, channelID)
		state.SetActive(false)
		state.ResetReconnectAttempts()
		
		// Disconnect if connected
		if vc, exists := b.session.VoiceConnections[guildID]; exists {
			delete(b.session.VoiceConnections, guildID)
			func() {
				defer func() { recover() }()
				_ = vc.Disconnect(b.ctx)
			}()
		}
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

	// Also check auto-connect channels
	autoConnectTicker := time.NewTicker(30 * time.Second)
	defer autoConnectTicker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			b.checkVoiceConnections()
		case <-autoConnectTicker.C:
			b.checkAutoConnectChannels()
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

		channelID := state.GetChannelID()
		if channelID == "" {
			continue
		}

		// Check if there are users in the channel before attempting reconnect
		userCount := b.countUsersInChannel(guildID, channelID)
		if userCount == 0 {
			b.logger.Infof("[%s] voice_check_loop: no users in channel %s, stopping radio", guildID, channelID)
			state.SetActive(false)
			state.ResetReconnectAttempts()
			
			// Disconnect if connected
			if vc, exists := b.session.VoiceConnections[guildID]; exists {
				delete(b.session.VoiceConnections, guildID)
				func() {
					defer func() { recover() }()
					_ = vc.Disconnect(b.ctx)
				}()
			}
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

// checkAutoConnectChannels checks saved auto-channels and connects if users are present
func (b *Bot) checkAutoConnectChannels() {
	// Get all guilds where bot is a member
	guilds := b.session.State.Guilds
	if len(guilds) == 0 {
		return
	}

	for _, guild := range guilds {
		guildID := guild.ID

		// Get or create state (this will load from config if exists)
		state := b.radioManager.GetOrCreate(guildID)

		// Check if auto-connect is enabled
		if !state.IsAutoConnectEnabled() {
			b.logger.Debugf("[%s] Auto-connect disabled, skipping", guildID)
			continue
		}

		autoChannelID := state.GetAutoChannelID()
		if autoChannelID == "" {
			// No auto-channel set for this guild
			b.logger.Debugf("[%s] No auto-channel set, skipping", guildID)
			continue
		}

		b.logger.Debugf("[%s] Checking auto-connect for channel %s", guildID, autoChannelID)

		// Check if radio is already active
		if state.IsActive() {
			// Check if we're in the right channel
			vc, exists := b.session.VoiceConnections[guildID]
			if exists && vc != nil && vc.Status == discordgo.VoiceConnectionStatusReady {
				// Check if we're in the correct channel
				botVS, err := b.session.State.VoiceState(guildID, b.session.State.User.ID)
				if err == nil && botVS != nil && botVS.ChannelID == autoChannelID {
					// Already connected to the right channel
					continue
				}
			}
		}

		// Count users in the auto-channel (excluding bots)
		// Use guild from loop, no need to fetch again
		userCount := 0
		for _, vs := range guild.VoiceStates {
			if vs.ChannelID == autoChannelID && vs.UserID != b.session.State.User.ID {
				// Check if user is a bot
				member, err := b.session.GuildMember(guildID, vs.UserID)
				if err == nil && member != nil && !member.User.Bot {
					userCount++
				}
			}
		}

		// Only auto-connect if there are users in the channel
		if userCount == 0 {
			b.logger.Debugf("[%s] No users in auto-channel %s, skipping", guildID, autoChannelID)
			continue
		}

		// Skip if already connecting (check if there's an active connection attempt)
		// This is handled by checking if radio is already active above

		// Auto-connect to the channel
		b.logger.Infof("[%s] Auto-connecting to saved channel %s (%d users present)", guildID, autoChannelID, userCount)

		b.wg.Add(1)
		go func(gid, cid string) {
			defer b.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					b.logger.WithField("panic", r).
						Errorf("[%s] Panic in auto-connect goroutine", gid)
				}
			}()

			state := b.radioManager.GetOrCreate(gid)
			state.SetActive(true)
			state.SetChannelID(cid)
			state.ResetReconnectAttempts()

			vc, err := b.connectToChannel(b.session, gid, cid)
			if err != nil {
				b.logger.WithError(err).Errorf("[%s] Failed to auto-connect to channel", gid)
				state.SetActive(false)
				return
			}

			if vc == nil {
				b.logger.Errorf("[%s] Voice connection is nil after auto-connect", gid)
				state.SetActive(false)
				return
			}

			// Start radio
			err = b.startRadio(vc, gid)
			if err != nil {
				b.logger.WithError(err).Errorf("[%s] Failed to start radio after auto-connect", gid)
				state.SetActive(false)
			}
		}(guildID, autoChannelID)
	}
}

// countUsersInChannel counts non-bot users in a voice channel
func (b *Bot) countUsersInChannel(guildID, channelID string) int {
	guild, err := b.session.Guild(guildID)
	if err != nil {
		b.logger.WithError(err).Debugf("[%s] Failed to get guild info", guildID)
		return 0
	}

	userCount := 0
	for _, vs := range guild.VoiceStates {
		if vs.ChannelID == channelID && vs.UserID != b.session.State.User.ID {
			// Check if user is a bot
			member, err := b.session.GuildMember(guildID, vs.UserID)
			if err == nil && member != nil && !member.User.Bot {
				userCount++
			}
		}
	}

	return userCount
}

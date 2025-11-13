package bot

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

// onReady handles the ready event
func (b *Bot) onReady(s *discordgo.Session, event *discordgo.Ready) {
	b.logger.Infof("Bot ready as %s (ID: %s)", event.User.Username, event.User.ID)

	// Start voice check loop
	b.wg.Add(1)
	go b.voiceCheckLoop()
}

// onMessageCreate handles message creation events
func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from bots
	if m.Author.Bot {
		return
	}

	// Check for command prefix
	if len(m.Content) == 0 || m.Content[0] != '!' {
		return
	}

	// Parse command - split by space to get command name
	parts := strings.Fields(m.Content[1:]) // Skip '!' and split by spaces
	if len(parts) == 0 {
		return
	}

	command := parts[0]

	// Handle commands
	switch command {
	case "join":
		b.handleJoin(s, m)
	case "radio":
		b.handleRadio(s, m)
	case "stop":
		b.handleStop(s, m)
	case "setchannel":
		b.handleSetChannel(s, m)
	case "autoconnect":
		b.handleAutoConnect(s, m)
	}
}

// onVoiceStateUpdate handles voice state updates
// Triggers auto-connect immediately when a user joins the configured channel
func (b *Bot) onVoiceStateUpdate(s *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
	// Ignore bot's own voice state changes
	if vs.UserID == s.State.User.ID {
		return
	}

	guildID := vs.GuildID
	
	// Get state for this guild
	state := b.radioManager.GetOrCreate(guildID)
	
	// Check if auto-connect is enabled
	if !state.IsAutoConnectEnabled() {
		return
	}

	autoChannelID := state.GetAutoChannelID()
	if autoChannelID == "" {
		// No auto-channel configured
		return
	}

	// Check if user joined a channel (Before == nil means user wasn't in a channel before)
	// After != nil means user is now in a channel
	if vs.BeforeUpdate == nil && vs.ChannelID != "" {
		// User joined a channel
		channelID := vs.ChannelID

		// Check if user joined the auto-channel
		if channelID != autoChannelID {
			return
		}

		// Get user to check if it's a bot
		member, err := s.GuildMember(guildID, vs.UserID)
		if err != nil {
			b.logger.WithError(err).Debugf("[%s] Failed to get member info", guildID)
			return
		}

		// Ignore bots
		if member.User.Bot {
			return
		}

		// Check if bot is already in this channel and active
		if state.IsActive() {
			botVS, err := s.State.VoiceState(guildID, s.State.User.ID)
			if err == nil && botVS != nil && botVS.ChannelID == channelID {
				// Bot is already in this channel and active
				return
			}
		}

		// Get channel info
		channel, err := s.Channel(channelID)
		if err != nil {
			b.logger.WithError(err).Debugf("[%s] Failed to get channel info", guildID)
			return
		}

		// Only join voice channels
		if channel.Type != discordgo.ChannelTypeGuildVoice {
			return
		}

		// Count users in channel (including the one who just joined)
		userCount := b.countUsersInChannel(guildID, channelID)
		if userCount == 0 {
			// Shouldn't happen, but just in case
			return
		}

		b.logger.Infof("[%s] User %s joined auto-channel %s, triggering immediate auto-connect", 
			guildID, member.User.Username, channel.Name)

		// Trigger auto-connect immediately in a goroutine
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
			
			// Double-check auto-connect is still enabled and channel matches
			if !state.IsAutoConnectEnabled() || state.GetAutoChannelID() != cid {
				return
			}

			// Check if already active
			if state.IsActive() {
				botVS, err := b.session.State.VoiceState(gid, b.session.State.User.ID)
				if err == nil && botVS != nil && botVS.ChannelID == cid {
					// Already connected
					return
				}
			}

			// Set state
			state.SetActive(true)
			state.SetChannelID(cid)
			state.ResetReconnectAttempts()

			// Connect to channel
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
	}(guildID, channelID)
	} else if vs.BeforeUpdate != nil && vs.BeforeUpdate.ChannelID != "" && vs.ChannelID == "" {
		// User left a channel (was in a channel before, now not in any channel)
		// Or user moved from one channel to another
		leftChannelID := vs.BeforeUpdate.ChannelID

		// Check if user left the auto-channel
		if leftChannelID != autoChannelID {
			return
		}

		// Get user to check if it's a bot
		member, err := s.GuildMember(guildID, vs.UserID)
		if err != nil {
			b.logger.WithError(err).Debugf("[%s] Failed to get member info", guildID)
			return
		}

		// Ignore bots
		if member.User.Bot {
			return
		}

		// Check if radio is active for this channel
		if !state.IsActive() || state.GetChannelID() != leftChannelID {
			return
		}

		// Count remaining users in channel
		userCount := b.countUsersInChannel(guildID, leftChannelID)
		if userCount == 0 {
			b.logger.Infof("[%s] Last user left channel %s, stopping radio", guildID, leftChannelID)
			
			// Stop radio
			state.SetActive(false)
			state.ResetReconnectAttempts()

			// Disconnect if connected
			if vc, exists := s.VoiceConnections[guildID]; exists {
				delete(s.VoiceConnections, guildID)
				func() {
					defer func() { recover() }()
					_ = vc.Disconnect(b.ctx)
				}()
			}
		}
	} else if vs.BeforeUpdate != nil && vs.BeforeUpdate.ChannelID != "" && vs.ChannelID != "" && vs.BeforeUpdate.ChannelID != vs.ChannelID {
		// User moved from one channel to another
		leftChannelID := vs.BeforeUpdate.ChannelID

		// Check if user left the auto-channel
		if leftChannelID == autoChannelID {
			// Get user to check if it's a bot
			member, err := s.GuildMember(guildID, vs.UserID)
			if err == nil && !member.User.Bot {
				// Check if radio is active for the channel user left
				if state.IsActive() && state.GetChannelID() == leftChannelID {
					// Count remaining users in the channel user left
					userCount := b.countUsersInChannel(guildID, leftChannelID)
					if userCount == 0 {
						b.logger.Infof("[%s] Last user left channel %s, stopping radio", guildID, leftChannelID)
						
						// Stop radio
						state.SetActive(false)
						state.ResetReconnectAttempts()

						// Disconnect if connected
						if vc, exists := s.VoiceConnections[guildID]; exists {
							delete(s.VoiceConnections, guildID)
							func() {
								defer func() { recover() }()
								_ = vc.Disconnect(b.ctx)
							}()
						}
					}
				}
			}
		}

		// Check if user joined the auto-channel (handled above in the first condition)
		// This is already handled by the first if statement, so we don't need to duplicate
	}
}


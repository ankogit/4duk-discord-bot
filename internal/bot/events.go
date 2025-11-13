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
	
	// Log all voice state updates for debugging
	var prevChan, currChan string
	if vs.BeforeUpdate != nil {
		prevChan = vs.BeforeUpdate.ChannelID
	}
	currChan = vs.ChannelID
	b.logger.Infof("[%s] Voice state update: user=%s, prev_channel=%s, curr_channel=%s", 
		guildID, vs.UserID, prevChan, currChan)
	
	// Check if auto-connect is enabled
	if !state.IsAutoConnectEnabled() {
		b.logger.Debugf("[%s] Auto-connect disabled, ignoring voice state update", guildID)
		return
	}

	autoChannelID := state.GetAutoChannelID()
	if autoChannelID == "" {
		// No auto-channel configured
		b.logger.Debugf("[%s] No auto-channel configured, ignoring voice state update", guildID)
		return
	}
	
	b.logger.Infof("[%s] Auto-channel is %s, checking if user joined it", guildID, autoChannelID)

	// Determine if user joined a channel
	// User joined if:
	// 1. BeforeUpdate is nil (bot didn't know about previous state) AND ChannelID is not empty
	// 2. BeforeUpdate.ChannelID is empty (user wasn't in a channel) AND ChannelID is not empty
	// 3. BeforeUpdate.ChannelID != ChannelID (user moved from one channel to another)
	var userJoinedChannel bool
	var previousChannelID string
	var currentChannelID string

	if vs.BeforeUpdate == nil {
		// Bot didn't know about previous state
		if vs.ChannelID != "" {
			userJoinedChannel = true
			currentChannelID = vs.ChannelID
		}
	} else {
		previousChannelID = vs.BeforeUpdate.ChannelID
		currentChannelID = vs.ChannelID
		
		// User joined if they weren't in a channel before and now are
		if previousChannelID == "" && currentChannelID != "" {
			userJoinedChannel = true
		}
		// User moved from one channel to another
		if previousChannelID != "" && currentChannelID != "" && previousChannelID != currentChannelID {
			userJoinedChannel = true
		}
	}

	// Log the result of userJoinedChannel check
	b.logger.Infof("[%s] userJoinedChannel=%v, currentChannelID=%s, autoChannelID=%s", 
		guildID, userJoinedChannel, currentChannelID, autoChannelID)

	// Check if user joined the auto-channel
	if userJoinedChannel && currentChannelID == autoChannelID {
		channelID := currentChannelID
		b.logger.Infof("[%s] User joined auto-channel %s, processing...", guildID, channelID)

		// Get user to check if it's a bot
		b.logger.Infof("[%s] Getting member info for user %s", guildID, vs.UserID)
		member, err := s.GuildMember(guildID, vs.UserID)
		if err != nil {
			b.logger.WithError(err).Warnf("[%s] Failed to get member info for user %s", guildID, vs.UserID)
			return
		}
		b.logger.Infof("[%s] Got member info: %s (bot: %v)", guildID, member.User.Username, member.User.Bot)

		// Ignore bots
		if member.User.Bot {
			b.logger.Infof("[%s] User %s is a bot, ignoring", guildID, member.User.Username)
			return
		}

		// Check if bot is already in this channel and active
		if state.IsActive() {
			botVS, err := s.State.VoiceState(guildID, s.State.User.ID)
			if err == nil && botVS != nil && botVS.ChannelID == channelID {
				// Bot is already in this channel and active
				b.logger.Infof("[%s] Bot is already in channel %s and active, skipping", guildID, channelID)
				return
			}
		}

		// Get channel info
		channel, err := s.Channel(channelID)
		if err != nil {
			b.logger.WithError(err).Warnf("[%s] Failed to get channel info", guildID)
			return
		}
		b.logger.Infof("[%s] Got channel info: %s (type: %d)", guildID, channel.Name, channel.Type)

		// Only join voice channels
		if channel.Type != discordgo.ChannelTypeGuildVoice {
			b.logger.Warnf("[%s] Channel %s is not a voice channel (type: %d), skipping", guildID, channel.Name, channel.Type)
			return
		}

		// Count users in channel using session state (more up-to-date than guild.VoiceStates)
		userCount := b.countUsersInChannelFromState(guildID, channelID)
		b.logger.Infof("[%s] User count in channel %s: %d", guildID, channel.Name, userCount)
		if userCount == 0 {
			// If countUsersInChannelFromState returns 0, but we know user just joined,
			// use the event data directly - at least 1 user (the one who just joined)
			b.logger.Infof("[%s] User count was 0, but user %s just joined, using count=1", guildID, member.User.Username)
			userCount = 1
		}

		b.logger.Infof("[%s] User %s joined auto-channel %s (%d users), triggering immediate auto-connect", 
			guildID, member.User.Username, channel.Name, userCount)

		// Trigger auto-connect immediately in a goroutine
		b.logger.Infof("[%s] Starting auto-connect goroutine for channel %s", guildID, channelID)
		b.wg.Add(1)
		go func(gid, cid string) {
			defer b.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					b.logger.WithField("panic", r).
						Errorf("[%s] Panic in auto-connect goroutine", gid)
				}
			}()

			b.logger.Infof("[%s] Auto-connect goroutine started", gid)
			state := b.radioManager.GetOrCreate(gid)
			
			// Double-check auto-connect is still enabled and channel matches
			if !state.IsAutoConnectEnabled() {
				b.logger.Warnf("[%s] Auto-connect disabled in goroutine, aborting", gid)
				return
			}
			if state.GetAutoChannelID() != cid {
				b.logger.Warnf("[%s] Auto-channel changed in goroutine (%s != %s), aborting", gid, state.GetAutoChannelID(), cid)
				return
			}

			// Check if already active
			if state.IsActive() {
				botVS, err := b.session.State.VoiceState(gid, b.session.State.User.ID)
				if err == nil && botVS != nil && botVS.ChannelID == cid {
					b.logger.Infof("[%s] Already connected to channel %s, skipping", gid, cid)
					return
				}
			}

			// Set state
			b.logger.Infof("[%s] Setting state: active=true, channel=%s", gid, cid)
			state.SetActive(true)
			state.SetChannelID(cid)
			state.ResetReconnectAttempts()

			// Connect to channel
			b.logger.Infof("[%s] Connecting to channel %s...", gid, cid)
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

			b.logger.Infof("[%s] Successfully connected to channel, starting radio...", gid)
			// Start radio
			err = b.startRadio(vc, gid)
			if err != nil {
				b.logger.WithError(err).Errorf("[%s] Failed to start radio after auto-connect", gid)
				state.SetActive(false)
			} else {
				b.logger.Infof("[%s] Radio started successfully after auto-connect", gid)
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

		// Count remaining users in channel (using more up-to-date state)
		userCount := b.countUsersInChannelFromState(guildID, leftChannelID)
		b.logger.Infof("[%s] User %s left channel %s, remaining users: %d", guildID, member.User.Username, leftChannelID, userCount)
		
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
		} else {
			b.logger.Infof("[%s] %d users still in channel %s, keeping radio", guildID, userCount, leftChannelID)
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
					// Count remaining users in the channel user left (using more up-to-date state)
					userCount := b.countUsersInChannelFromState(guildID, leftChannelID)
					b.logger.Infof("[%s] User %s moved from channel %s, remaining users: %d", guildID, member.User.Username, leftChannelID, userCount)
					
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
					} else {
						b.logger.Infof("[%s] %d users still in channel %s, keeping radio", guildID, userCount, leftChannelID)
					}
				}
			}
		}

		// Check if user joined the auto-channel (handled above in the first condition)
		// This is already handled by the first if statement, so we don't need to duplicate
	}
}


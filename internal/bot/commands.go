package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// handleJoin handles the !join command
func (b *Bot) handleJoin(s *discordgo.Session, m *discordgo.MessageCreate) {
	guildID := m.GuildID
	channelID := m.ChannelID

	// Check if user is in a voice channel
	vs, err := s.State.VoiceState(m.GuildID, m.Author.ID)
	if err != nil || vs == nil {
		s.ChannelMessageSend(channelID, "Ты не в голосовом канале!")
		return
	}

	channel, err := s.Channel(vs.ChannelID)
	if err != nil {
		b.logger.WithError(err).Errorf("[%s] Failed to get channel", guildID)
		s.ChannelMessageSend(channelID, "Ошибка при получении информации о канале.")
		return
	}

	if channel.Type != discordgo.ChannelTypeGuildVoice {
		s.ChannelMessageSend(channelID, "Это не голосовой канал!")
		return
	}

	state := b.radioManager.GetOrCreate(guildID)
	state.SetChannelID(vs.ChannelID)

	vc, err := b.connectToChannel(s, m.GuildID, vs.ChannelID)
	if err != nil {
		b.logger.WithError(err).Errorf("[%s] Failed to connect to channel", guildID)
		s.ChannelMessageSend(channelID, fmt.Sprintf("Не удалось подключиться к голосовому каналу: %v", err))
		return
	}

	if vc != nil {
		s.ChannelMessageSend(channelID, fmt.Sprintf("Подключился к %s", channel.Name))
	}
}

// handleRadio handles the !radio command
func (b *Bot) handleRadio(s *discordgo.Session, m *discordgo.MessageCreate) {
	guildID := m.GuildID
	channelID := m.ChannelID

	// Check if user is in a voice channel
	vs, err := s.State.VoiceState(m.GuildID, m.Author.ID)
	if err != nil || vs == nil {
		s.ChannelMessageSend(channelID, "Ты не в голосовом канале!")
		return
	}

	channel, err := s.Channel(vs.ChannelID)
	if err != nil {
		b.logger.WithError(err).Errorf("[%s] Failed to get channel", guildID)
		s.ChannelMessageSend(channelID, "Ошибка при получении информации о канале.")
		return
	}

	if channel.Type != discordgo.ChannelTypeGuildVoice {
		s.ChannelMessageSend(channelID, "Это не голосовой канал!")
		return
	}

	state := b.radioManager.GetOrCreate(guildID)
	state.SetActive(true)
	state.SetChannelID(vs.ChannelID)
	state.ResetReconnectAttempts()

	vc, err := b.connectToChannel(s, m.GuildID, vs.ChannelID)
	if err != nil {
		b.logger.WithError(err).Errorf("[%s] Failed to connect to channel", guildID)
		s.ChannelMessageSend(channelID, "Не удалось подключиться к голосовому каналу для радио.")
		return
	}

	if vc == nil {
		s.ChannelMessageSend(channelID, "Не удалось подключиться к голосовому каналу для радио.")
		return
	}

	err = b.startRadio(vc, guildID)
	if err != nil {
		b.logger.WithError(err).Errorf("[%s] Failed to start radio", guildID)
		s.ChannelMessageSend(channelID, fmt.Sprintf("Ошибка при запуске радио: %v", err))
		return
	}

	s.ChannelMessageSend(channelID, "🎵 Вещаю радио!")
}

// handleStop handles the !stop command
func (b *Bot) handleStop(s *discordgo.Session, m *discordgo.MessageCreate) {
	guildID := m.GuildID
	channelID := m.ChannelID

	state := b.radioManager.GetOrCreate(guildID)
	state.SetActive(false)
	state.ResetReconnectAttempts()

	vc, exists := s.VoiceConnections[guildID]
	if !exists || vc == nil {
		s.ChannelMessageSend(channelID, "Я не в голосовом канале.")
		return
	}

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

	// Cleanup encoder
	b.encoderPool.Remove(guildID)

	s.ChannelMessageSend(channelID, "Отключился.")
}

// handleSetChannel handles the !setchannel command
// Sets the channel for auto-join when users are present
func (b *Bot) handleSetChannel(s *discordgo.Session, m *discordgo.MessageCreate) {
	guildID := m.GuildID
	textChannelID := m.ChannelID

	// Parse command arguments
	parts := strings.Fields(m.Content)
	if len(parts) < 2 {
		s.ChannelMessageSend(textChannelID, "Использование: `!setchannel <ID_канала>`")
		return
	}

	// Extract channel ID (second part after command)
	channelID := parts[1]

	// Get channel info to verify it exists and is a voice channel
	channel, err := s.Channel(channelID)
	if err != nil {
		b.logger.WithError(err).Debugf("[%s] Failed to get channel info", guildID)
		s.ChannelMessageSend(textChannelID, "Канал не найден. Проверьте ID канала.")
		return
	}

	if channel.Type != discordgo.ChannelTypeGuildVoice {
		s.ChannelMessageSend(textChannelID, "Это не голосовой канал!")
		return
	}

	// Save auto-channel
	state := b.radioManager.GetOrCreate(guildID)
	state.SetAutoChannelID(channelID)
	// Enable auto-connect when setting channel
	state.SetAutoConnectEnabled(true)
	b.radioManager.SaveState(guildID)

	s.ChannelMessageSend(textChannelID, fmt.Sprintf("✅ Авто-подключение установлено на канал: **%s** (включено)", channel.Name))
	b.logger.Infof("[%s] Auto-channel set to %s (%s)", guildID, channel.Name, channelID)
}

// handleAutoConnect handles the !autoconnect command
// Enables or disables auto-connect feature
func (b *Bot) handleAutoConnect(s *discordgo.Session, m *discordgo.MessageCreate) {
	guildID := m.GuildID
	textChannelID := m.ChannelID

	// Parse command arguments
	parts := strings.Fields(m.Content)
	if len(parts) < 2 {
		state := b.radioManager.GetOrCreate(guildID)
		enabled := state.IsAutoConnectEnabled()
		autoChannelID := state.GetAutoChannelID()

		status := "выключено"
		if enabled {
			status = "включено"
		}

		message := fmt.Sprintf("Авто-подключение: **%s**", status)

		if autoChannelID != "" {
			// Get channel info
			channel, err := s.Channel(autoChannelID)
			if err == nil && channel != nil {
				message += fmt.Sprintf("\nКанал: **%s** (`%s`)", channel.Name, autoChannelID)
			} else {
				message += fmt.Sprintf("\nКанал: `%s` (канал не найден)", autoChannelID)
			}
		} else {
			message += "\nКанал: не установлен"
		}

		message += "\n\nИспользование: `!autoconnect on` или `!autoconnect off`"
		s.ChannelMessageSend(textChannelID, message)
		return
	}

	action := strings.ToLower(parts[1])
	state := b.radioManager.GetOrCreate(guildID)

	switch action {
	case "on", "enable", "вкл", "да":
		state.SetAutoConnectEnabled(true)
		b.radioManager.SaveState(guildID)
		autoChannelID := state.GetAutoChannelID()
		if autoChannelID != "" {
			channel, _ := s.Channel(autoChannelID)
			channelName := autoChannelID
			if channel != nil {
				channelName = channel.Name
			}
			s.ChannelMessageSend(textChannelID, fmt.Sprintf("✅ Авто-подключение **включено** для канала: **%s**", channelName))
		} else {
			s.ChannelMessageSend(textChannelID, "✅ Авто-подключение **включено**. Установите канал командой `!setchannel <ID>`")
		}
	case "off", "disable", "выкл", "нет":
		state.SetAutoConnectEnabled(false)
		b.radioManager.SaveState(guildID)
		s.ChannelMessageSend(textChannelID, "❌ Авто-подключение **выключено**")
	default:
		s.ChannelMessageSend(textChannelID, "Использование: `!autoconnect on` или `!autoconnect off`")
	}
}

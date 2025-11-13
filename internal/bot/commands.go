package bot

import (
	"context"
	"fmt"

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

	err := vc.Disconnect(context.Background())
	if err != nil {
		b.logger.WithError(err).Errorf("[%s] Failed to disconnect", guildID)
		s.ChannelMessageSend(channelID, fmt.Sprintf("Ошибка при отключении: %v", err))
		return
	}

	// Cleanup encoder
	b.encoderPool.Remove(guildID)

	s.ChannelMessageSend(channelID, "Отключился.")
}


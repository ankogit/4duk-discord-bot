package bot

import (
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

	// Parse command
	var command string
	if len(m.Content) > 1 {
		command = m.Content[1:]
	} else {
		return
	}

	// Handle commands
	switch command {
	case "join":
		b.handleJoin(s, m)
	case "radio":
		b.handleRadio(s, m)
	case "stop":
		b.handleStop(s, m)
	}
}


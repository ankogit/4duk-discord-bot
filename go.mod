module github.com/ankogit/4duk-discord-bot

go 1.21

require (
	github.com/bwmarrin/discordgo v0.29.0
	github.com/hraban/opus v0.0.0-20230925203106-0188a62cb302
	github.com/joho/godotenv v1.5.1
	github.com/sirupsen/logrus v1.9.3
)

// Use fork with voice overhaul PR #1593 to fix "Unknown encryption mode" error
// This PR adds support for new encryption modes required by Discord since Nov 18, 2024
// See: https://github.com/bwmarrin/discordgo/pull/1593
// Note: Fork has a bug with nil channel panic, we handle it with recover blocks
replace github.com/bwmarrin/discordgo => github.com/ozraru/discordgo v0.26.2-0.20251101184423-6792228f3271

require (
	github.com/gorilla/websocket v1.5.1 // indirect
	golang.org/x/crypto v0.18.0 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
)

// Copyright (C) 2025 Rafael Galvan and contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package bot

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"

	"rorbotgo/config"
	"rorbotgo/internal/database"
	"rorbotgo/server"
)

// Bot holds the Discord session and coordinates startup and shutdown.
type Bot struct {
	// session is the Discord session used to interact with the Discord API.
	session *discordgo.Session
	// db is the database used to store server state.
	db *database.Database
	// manager is the server manager used to handle Discord events.
	manager *server.Manager
	// registeredCmds is a map of registered slash commands, keyed by command name.
	registeredCmds map[string][]*discordgo.ApplicationCommand
}

// NewBot creates a Bot from the provided configuration and database.
func NewBot(cfg *config.Configuration, db *database.Database) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		return nil, err
	}
	session.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentMessageContent

	b := &Bot{
		session:        session,
		db:             db,
		registeredCmds: make(map[string][]*discordgo.ApplicationCommand),
	}
	b.manager = server.NewManager(session, db)
	b.manager.RegisterHandlers()

	return b, nil
}

// Start opens the Discord gateway, registers slash commands.
func (b *Bot) Start() error {
	if err := b.session.Open(); err != nil {
		return err
	}
	defer b.session.Close()

	if err := b.registerCommands(); err != nil {
		return err
	}
	defer b.cleanupCommands()

	slog.Info("discord bot is running")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	<-sc

	slog.Info("shutting down")
	b.manager.DisconnectAll()
	return nil
}

// registerCommands registers slash commands as application commands.
// Set RORBOT_GUILD_ID to a guild ID for instant guild-scoped registration
// during development (global commands can take up to an hour to propagate).
func (b *Bot) registerCommands() error {
	guildID := os.Getenv("RORBOT_GUILD_ID")
	var registered []*discordgo.ApplicationCommand
	for _, cmd := range b.manager.Commands() {
		reg, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, guildID, cmd)
		if err != nil {
			return err
		}
		registered = append(registered, reg)
		slog.Debug("registered slash command", "name", cmd.Name, "guild", guildID)
	}
	b.registeredCmds[guildID] = registered
	return nil
}

// cleanupCommands deletes all registered slash commands.
func (b *Bot) cleanupCommands() {
	for guildID, cmds := range b.registeredCmds {
		for _, cmd := range cmds {
			if err := b.session.ApplicationCommandDelete(b.session.State.User.ID, guildID, cmd.ID); err != nil {
				slog.Warn("failed to delete slash command", "name", cmd.Name, "err", err)
			}
		}
	}
}

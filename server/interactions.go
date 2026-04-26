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

package server

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

const (
	CmdCreateServer = "create_server"
	CmdConnect      = "connect"
	CmdDisconnect   = "disconnect"
	CmdDeleteServer = "delete_server"
	CmdListServers  = "list_servers"
)

// handleInteraction will dispatch incoming interactions to the handler based
// on the command name.
func (m *Manager) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// we only care about slash commands for now
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	// data is the same for both application command and message component
	// interactions, so we can extract it here
	data := i.ApplicationCommandData()
	slog.Debug("slash command",
		"command", data.Name,
		"guild_id", i.GuildID,
		"channel_id", i.ChannelID,
		"user", interactionUser(i),
	)
	switch data.Name {
	case CmdCreateServer:
		m.handleCreateServer(s, i, data)
	case CmdConnect:
		m.handleConnect(s, i)
	case CmdDisconnect:
		m.handleDisconnect(s, i)
	case CmdDeleteServer:
		m.handleDeleteServer(s, i)
	case CmdListServers:
		m.handleListServers(s, i)
	default:
		slog.Warn("unhandled slash command", "command", data.Name)
	}
}

// handleMessageCreate relays messages typed in a linked Discord channel to the
// corresponding server.
func (m *Manager) handleMessageCreate(s *discordgo.Session, msg *discordgo.MessageCreate) {
	// ignore messages from bots (including itself)
	if msg.Author == nil || msg.Author.Bot {
		return
	}

	channelID, _ := strconv.ParseInt(msg.ChannelID, 10, 64)

	m.mu.RLock()
	server, ok := m.servers[channelID]
	m.mu.RUnlock()

	// If we don't have a server linked to this channel, ignore the message.
	if !ok {
		return
	}

	content := fmt.Sprintf("[Discord/%s] %s", msg.Author.Username, msg.Content)
	slog.Debug("relaying discord message to server",
		"name", server.Model.Name,
		"author", msg.Author.Username,
		"message", msg.Content,
	)

	if err := server.SendChat(content); err != nil {
		slog.Warn("failed to relay discord message to server",
			"name", server.Model.Name,
			"author", msg.Author.Username,
			"err", err,
		)
	}
}

// interactionUser returns the username of whoever triggered an interaction.
func interactionUser(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.Username
	}
	if i.User != nil {
		return i.User.Username
	}
	return "unknown"
}

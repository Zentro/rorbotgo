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
	"log/slog"
	"sync"

	"github.com/bwmarrin/discordgo"

	"rorbotgo/internal/database"
)

// Manager tracks all active server connections and handles the Discord
// slash commands that create, connect, disconnect, and delete them.
type Manager struct {
	session *discordgo.Session
	db      *database.Database

	mu      sync.RWMutex
	servers map[int64]*Server // keyed by Discord channel ID
}

// NewManager creates a Manager. Call RegisterHandlers before opening the session.
func NewManager(session *discordgo.Session, db *database.Database) *Manager {
	slog.Debug("initializing server manager")
	return &Manager{
		session: session,
		db:      db,
		servers: make(map[int64]*Server),
	}
}

// RegisterHandlers attaches all Discord event handlers to the session.
func (m *Manager) RegisterHandlers() {
	slog.Debug("registering discord event handlers")
	m.session.AddHandler(m.handleInteraction)
	m.session.AddHandler(m.handleMessageCreate)
}

// Commands returns the slash command definitions to register with Discord.
func (m *Manager) Commands() []*discordgo.ApplicationCommand {
	minPort := 1.0
	maxPort := 65535.0
	return []*discordgo.ApplicationCommand{
		{
			Name:        "create_server",
			Description: "Register a server with this channel",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "name", Description: "Display name for this server", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "host", Description: "Hostname or IP of the RoR server", Required: true},
				{Type: discordgo.ApplicationCommandOptionInteger, Name: "port", Description: "Port number (default 12000)", Required: false,
					MinValue: &minPort, MaxValue: maxPort},
				{Type: discordgo.ApplicationCommandOptionString, Name: "password", Description: "Server password (if any)", Required: false},
			},
		},
		{
			Name:        "connect",
			Description: "Connect to the server linked to this channel",
		},
		{
			Name:        "disconnect",
			Description: "Disconnect from the server linked to this channel",
		},
		{
			Name:        "delete_server",
			Description: "Remove the server registration for this channel",
		},
		{
			Name:        "list_servers",
			Description: "List all registered servers in this guild",
		},
		{
			Name:                     "exec",
			Description:              "Execute a command on the server from this guild",
			DefaultMemberPermissions: func(i int64) *int64 { return &i }(discordgo.PermissionAdministrator),
		},
	}
}

// DisconnectAll cleanly stops every active server connection.
func (m *Manager) DisconnectAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	slog.Info("disconnecting", "count", len(m.servers))
	for _, s := range m.servers {
		s.Disconnect()
	}
	m.servers = make(map[int64]*Server)
}

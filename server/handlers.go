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
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (m *Manager) handleCreateServer(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	opts := optionMap(data.Options)

	name := opts["name"].StringValue()
	host := opts["host"].StringValue()
	port := 12000
	if p, ok := opts["port"]; ok {
		port = int(p.IntValue())
	}
	password := ""
	if pw, ok := opts["password"]; ok {
		password = pw.StringValue()
	}

	guildID, _ := strconv.ParseInt(i.GuildID, 10, 64)
	channelID, _ := strconv.ParseInt(i.ChannelID, 10, 64)

	slog.Debug("create_server invoked",
		"name", name,
		"host", host,
		"port", port,
		"guild_id", guildID,
		"channel_id", channelID,
	)

	if _, err := m.db.GetServerByChannel(channelID); err == nil {
		slog.Warn("create_server rejected: channel already has a server",
			"channel_id", channelID,
		)
		respond(s, i, "This channel already has a server registered. Use `/delete_server` first.")
		return
	}

	srv, err := m.db.CreateServer(name, guildID, channelID, host, port, password)
	if err != nil {
		slog.Error("failed to create server record",
			"name", name,
			"host", host,
			"port", port,
			"err", err,
		)
		respond(s, i, fmt.Sprintf("Failed to register server: %s", err))
		return
	}

	slog.Info("server registered",
		"id", srv.ID,
		"name", srv.Name,
		"host", srv.Host,
		"port", srv.Port,
		"channel_id", channelID,
	)
	respond(s, i, fmt.Sprintf("Registered **%s** (`%s:%d`) to this channel (ID: %d). Use `/connect` to connect.", srv.Name, srv.Host, srv.Port, srv.ID))
}

func (m *Manager) handleConnect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelID, _ := strconv.ParseInt(i.ChannelID, 10, 64)

	m.mu.RLock()
	_, active := m.servers[channelID]
	m.mu.RUnlock()
	if active {
		slog.Debug("connect rejected: already active", "channel_id", channelID)
		respond(s, i, "Already connected to a server on this channel.")
		return
	}

	slog.Debug("connect invoked", "channel_id", channelID)

	model, err := m.db.GetServerByChannel(channelID)
	if err == sql.ErrNoRows {
		slog.Warn("connect rejected: no server registered", "channel_id", channelID)
		respond(s, i, "No server registered for this channel. Use `/create_server` first.")
		return
	} else if err != nil {
		slog.Error("db error looking up server for connect", "channel_id", channelID, "err", err)
		respond(s, i, fmt.Sprintf("Database error: %s", err))
		return
	}

	// Acknowledge immediately as the TCP handshake may take several seconds and
	// Discord invalidates the interaction token after 3ish seconds.
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); err != nil {
		slog.Warn("failed to defer connect interaction", "channel_id", channelID, "err", err)
		return
	}

	slog.Info("connecting to server",
		"name", model.Name,
		"host", model.Host,
		"port", model.Port,
		"channel_id", channelID,
	)

	go func() {
		srv, err := NewServer(model, s, func() {
			slog.Info("removing server from active map after unexpected disconnect",
				"name", model.Name,
				"channel_id", channelID,
			)
			m.mu.Lock()
			delete(m.servers, channelID)
			m.mu.Unlock()
		})

		var content string
		if err != nil {
			slog.Error("server connection failed",
				"name", model.Name,
				"host", model.Host,
				"port", model.Port,
				"err", err,
			)
			content = fmt.Sprintf("Failed to connect to **%s**: %s", model.Name, err)
		} else {
			m.mu.Lock()
			m.servers[channelID] = srv
			m.mu.Unlock()
			slog.Info("server connection established",
				"name", model.Name,
				"channel_id", channelID,
			)
			content = fmt.Sprintf("Connected to **%s** (`%s:%d`).", model.Name, model.Host, model.Port)
		}

		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		}); err != nil {
			slog.Warn("failed to edit deferred connect response",
				"channel_id", channelID,
				"err", err,
			)
		}
	}()
}

func (m *Manager) handleDisconnect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelID, _ := strconv.ParseInt(i.ChannelID, 10, 64)

	slog.Debug("disconnect invoked", "channel_id", channelID)

	m.mu.Lock()
	srv, ok := m.servers[channelID]
	if ok {
		delete(m.servers, channelID)
	}
	m.mu.Unlock()

	if !ok {
		slog.Warn("disconnect rejected: not connected", "channel_id", channelID)
		respond(s, i, "Not connected to any server on this channel.")
		return
	}

	slog.Info("disconnecting server by request",
		"name", srv.Model.Name,
		"channel_id", channelID,
	)
	srv.Disconnect()
	respond(s, i, "Disconnected from server.")
}

func (m *Manager) handleDeleteServer(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelID, _ := strconv.ParseInt(i.ChannelID, 10, 64)

	slog.Debug("delete_server invoked", "channel_id", channelID)

	// Force disconnect if active.
	m.mu.Lock()
	if srv, ok := m.servers[channelID]; ok {
		slog.Info("force-disconnecting server before deletion",
			"name", srv.Model.Name,
			"channel_id", channelID,
		)
		delete(m.servers, channelID)
		srv.Disconnect()
	}
	m.mu.Unlock()

	model, err := m.db.GetServerByChannel(channelID)
	if err == sql.ErrNoRows {
		slog.Warn("delete_server rejected: no server registered", "channel_id", channelID)
		respond(s, i, "No server registered for this channel.")
		return
	} else if err != nil {
		slog.Error("db error looking up server for deletion", "channel_id", channelID, "err", err)
		respond(s, i, fmt.Sprintf("Database error: %s", err))
		return
	}

	if err := m.db.DeleteServer(model.ID); err != nil {
		slog.Error("failed to delete server record", "id", model.ID, "name", model.Name, "err", err)
		respond(s, i, fmt.Sprintf("Failed to delete server: %s", err))
		return
	}

	slog.Info("server deleted", "id", model.ID, "name", model.Name, "channel_id", channelID)
	respond(s, i, fmt.Sprintf("Removed server **%s** from this channel.", model.Name))
}

func (m *Manager) handleListServers(s *discordgo.Session, i *discordgo.InteractionCreate) {
	guildID, _ := strconv.ParseInt(i.GuildID, 10, 64)

	slog.Debug("list_servers invoked", "guild_id", guildID)

	servers, err := m.db.ListServers(guildID)
	if err != nil {
		slog.Error("db error listing servers", "guild_id", guildID, "err", err)
		respond(s, i, fmt.Sprintf("Database error: %s", err))
		return
	}

	slog.Debug("list_servers result", "guild_id", guildID, "count", len(servers))

	if len(servers) == 0 {
		respond(s, i, "No servers registered in this guild.")
		return
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var msg strings.Builder
	msg.WriteString("**Registered servers:**\n")
	for _, sv := range servers {
		status := "disconnected"
		if _, ok := m.servers[sv.ChannelID]; ok {
			status = "connected"
		}
		fmt.Fprintf(&msg, "• **%s** — `%s:%d` — <#%d> — %s\n", sv.Name, sv.Host, sv.Port, sv.ChannelID, status)
	}
	respond(s, i, msg.String())
}

func optionMap(opts []*discordgo.ApplicationCommandInteractionDataOption) map[string]*discordgo.ApplicationCommandInteractionDataOption {
	m := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(opts))
	for _, o := range opts {
		m[o.Name] = o
	}
	return m
}

func respond(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content},
	}); err != nil {
		slog.Warn("failed to respond to interaction",
			"command", i.ApplicationCommandData().Name,
			"channel_id", i.ChannelID,
			"err", err,
		)
	}
}

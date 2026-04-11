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
	"sync"

	"github.com/bwmarrin/discordgo"

	"rorbotgo/config"
	"rorbotgo/internal/models"
	"rorbotgo/rornet2"
)

// Server represents one active RoR server connection and its Discord binding.
// Each instance runs its own goroutine (via relayEvents) that forwards RoRnet
// events to the associated Discord channel.
type Server struct {
	Model        *models.Server
	client       *rornet2.Client
	session      *discordgo.Session
	onDisconnect func() // called by relayEvents on unexpected server-side disconnect

	usersMu sync.RWMutex
	users   map[uint32]string // uniqueID → username
}

// newServer creates a Server, connects the RoRnet client, and starts the event
// relay goroutine. onDisconnect is invoked when the server drops the connection
// without a user-initiated /disconnect (e.g. crash, timeout). Returns an error
// if the TCP handshake fails.
func newServer(model *models.Server, session *discordgo.Session, onDisconnect func()) (*Server, error) {
	slog.Debug("connecting to server",
		"name", model.Name,
		"host", model.Host,
		"port", model.Port,
		"channel_id", model.ChannelID,
	)

	cfg := config.Get().Bot

	client := rornet2.NewClient(model.Host, model.Port, cfg.Username, model.Password, cfg.Language, cfg.Token)
	if err := client.Connect(); err != nil {
		slog.Error("failed to connect to server",
			"name", model.Name,
			"host", model.Host,
			"port", model.Port,
			"err", err,
		)
		return nil, err
	}

	slog.Info("server connected",
		"name", model.Name,
		"host", model.Host,
		"port", model.Port,
		"channel_id", model.ChannelID,
	)

	s := &Server{
		Model:        model,
		client:       client,
		session:      session,
		onDisconnect: onDisconnect,
		users:        make(map[uint32]string),
	}
	go s.relayEvents()
	return s, nil
}

// Disconnect cleanly shuts down the RoRnet connection.
func (s *Server) Disconnect() {
	slog.Info("disconnecting from server",
		"name", s.Model.Name,
		"host", s.Model.Host,
		"port", s.Model.Port,
	)
	s.client.Disconnect()
}

// IsConnected reports whether the underlying RoRnet client is connected.
func (s *Server) IsConnected() bool {
	return s.client.IsConnected()
}

func (s *Server) SendCommand(cmd string) error {
	return nil
}

// SendChat forwards a chat message from Discord to the RoR server.
func (s *Server) SendChat(message string) error {
	slog.Debug("sending chat to RoR server", "name", s.Model.Name, "message", message)
	return s.client.SendChat(message)
}

// relayEvents drains the rornet2 event channel and posts messages to Discord.
// Runs for the lifetime of the connection; returns when the client disconnects.
func (s *Server) relayEvents() {
	channelID := fmt.Sprintf("%d", s.Model.ChannelID)
	name := s.Model.Name

	slog.Debug("relay goroutine started", "name", name, "channel_id", s.Model.ChannelID)

	for event := range s.client.Events {
		switch event.Kind {
		case rornet2.EventDisconnect:
			if event.Err != nil {
				slog.Warn("server connection lost",
					"name", name,
					"err", event.Err,
				)
				s.chanSend(channelID, fmt.Sprintf("**[%s]** Connection lost — %s", name, event.Err))
			} else {
				slog.Info("server disconnected cleanly without errors", "name", name)
				s.chanSend(channelID, fmt.Sprintf("**[%s]** Disconnected from RoR server.", name))
			}
			// Notify the Manager so it removes this entry from its active map.
			// This covers unexpected drops (crash, timeout, server shutdown)
			// where the user never ran /disconnect.
			if s.onDisconnect != nil {
				s.onDisconnect()
			}
			slog.Debug("relay goroutine exiting", "name", name)
			return

		case rornet2.EventUserSync:
			if event.UserInfo != nil {
				uname := rornet2.CString(event.UserInfo.Username[:])
				s.storeUser(uint32(event.Source), event.UserInfo.UniqueID, uname)
				slog.Debug("synced existing user",
					"name", name,
					"source", event.Source,
					"unique_id", event.UserInfo.UniqueID,
					"username", uname,
				)
			}

		case rornet2.EventMessage:
			if event.Message == "" {
				continue
			}
			who := s.username(uint32(event.Source))
			slog.Debug("relaying chat message",
				"name", name,
				"source", event.Source,
				"who", who,
				"message", event.Message,
			)
			s.chanSend(channelID, fmt.Sprintf("**[%s]** %s: %s", name, who, event.Message))

		case rornet2.EventUserJoin:
			playerName := fmt.Sprintf("User %d", event.Source)
			if event.UserInfo != nil {
				playerName = rornet2.CString(event.UserInfo.Username[:])
				s.storeUser(uint32(event.Source), event.UserInfo.UniqueID, playerName)
			}
			slog.Info("player joined server",
				"name", name,
				"player", playerName,
				"source", event.Source,
			)
			s.chanSend(channelID, fmt.Sprintf("**[%s]** :green_circle: **%s** joined.", name, playerName))

		case rornet2.EventUserLeave:
			playerName := s.username(uint32(event.Source))
			s.usersMu.Lock()
			delete(s.users, uint32(event.Source))
			s.usersMu.Unlock()
			slog.Info("player left RoR server",
				"name", name,
				"player", playerName,
				"source", event.Source,
			)
			s.chanSend(channelID, fmt.Sprintf("**[%s]** :red_circle: **%s** left.", name, playerName))

		case rornet2.EventError:
			slog.Warn("unexpected server error", "name", name, "err", event.Err)
		}
	}

	slog.Debug("relay goroutine: event channel closed", "name", name)
}

// storeUser writes a username under both the header source ID and the struct
// UniqueID, because different server versions populate different fields.
// Zero values are skipped.
func (s *Server) storeUser(source, uniqueID uint32, name string) {
	s.usersMu.Lock()
	defer s.usersMu.Unlock()
	if source != 0 {
		s.users[source] = name
	}
	if uniqueID != 0 && uniqueID != source {
		s.users[uniqueID] = name
	}
}

// username looks up a player's name by their unique ID, falling back to
// "User <id>" if they are not in the map (e.g. system messages, unknown source).
func (s *Server) username(id uint32) string {
	if id == 0 || int32(id) < 0 {
		return "Server"
	}
	s.usersMu.RLock()
	name, ok := s.users[id]
	s.usersMu.RUnlock()
	if ok {
		return name
	}
	return fmt.Sprintf("User %d", id)
}

// chanSend sends a message to a Discord channel and logs any error.
func (s *Server) chanSend(channelID, content string) {
	if _, err := s.session.ChannelMessageSend(channelID, content); err != nil {
		slog.Error("failed to send discord message",
			"name", s.Model.Name,
			"channel_id", channelID,
			"err", err,
		)
	}
}

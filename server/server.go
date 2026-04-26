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

	"rorbotgo/client"
	"rorbotgo/config"
	"rorbotgo/internal/models"
)

// Server represents one active RoR server connection and its Discord binding.
// Each instance runs its own goroutine (via listenForServerEvents) that forwards RoRnet
// events to the associated Discord channel.
type Server struct {
	Model        *models.Server
	client       *client.Client
	session      *discordgo.Session
	onDisconnect func() // called by listenForServerEvents on unexpected server-side disconnect

	usersLock sync.RWMutex
	users     map[uint32]string // uniqueID → username
}

// NewServer creates a Server, connects the client, and starts the event
// relay goroutine. onDisconnect is invoked when the server drops the connection
// without a user-initiated /disconnect (e.g. crash, timeout). Returns an error
// if the TCP handshake fails.
func NewServer(model *models.Server, session *discordgo.Session, onDisconnect func()) (*Server, error) {
	slog.Debug("connecting to server",
		"name", model.Name,
		"host", model.Host,
		"port", model.Port,
		"channel_id", model.ChannelID,
	)

	cfg := config.Get().Bot

	client := client.NewClient(model.Host, model.Port, cfg.Username, model.Password, cfg.Language, cfg.Token)
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
	go s.listenForServerEvents()
	return s, nil
}

func (s *Server) ID() string {
	return fmt.Sprintf("%d", s.Model.ID)
}

func (s *Server) ChannelID() string {
	return fmt.Sprintf("%d", s.Model.ChannelID)
}

func (s *Server) Log() *slog.Logger {
	return slog.With("server", s.ID())
}

func (s *Server) Disconnect() {
	s.Log().Info("disconnecting from server",
		"name", s.Model.Name,
		"host", s.Model.Host,
		"port", s.Model.Port,
	)
	s.client.Disconnect()
}

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

// storeUser writes a username under both the header source ID and the struct
// UniqueID, because different server versions populate different fields.
// Zero values are skipped.
func (s *Server) storeUser(source, uniqueID uint32, name string) {
	s.usersLock.Lock()
	defer s.usersLock.Unlock()
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
	s.usersLock.RLock()
	name, ok := s.users[id]
	s.usersLock.RUnlock()
	if ok {
		return name
	}
	return fmt.Sprintf("User %d", id)
}

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
	"rorbotgo/client"
)

// listenForServerEvents drains the server's event channel and processes each event
// and sends them along to Discord. This function will run until the channel is
// closed, either by an expected disconnect (e.g. user-initiated /disconnect) or
// an unexpected one (e.g. crash, timeout). In the case of an unexpected disconnect,
// the onDisconnect callback will be invoked to notify the Manager so it can remove
// this Server from its active map.
func (s *Server) listenForServerEvents() {
	name := s.Model.Name

	slog.Debug("listening for server events", "name", name, "channel_id", s.Model.ChannelID)

	for event := range s.client.Events {
		switch event.Kind {
		case client.EventDisconnect:
			if event.Err != nil {
				slog.Warn("server connection lost",
					"name", name,
					"err", event.Err,
				)
				s.SendChannelMessage(fmt.Sprintf("**[%s]** Connection lost — %s", name, event.Err))
			} else {
				slog.Info("server disconnected cleanly without errors", "name", name)
				s.SendChannelMessage(fmt.Sprintf("**[%s]** Disconnected from RoR server.", name))
			}
			// Notify the Manager so it removes this entry from its active map.
			if s.onDisconnect != nil {
				s.onDisconnect()
			}
			slog.Debug("relay goroutine exiting", "name", name)
			return

		case client.EventUserSync:
			if event.UserInfo != nil {
				uname := client.CString(event.UserInfo.Username[:])
				s.storeUser(uint32(event.Source), event.UserInfo.UniqueID, uname)
				slog.Debug("synced existing user",
					"name", name,
					"source", event.Source,
					"unique_id", event.UserInfo.UniqueID,
					"username", uname,
				)
			}

		case client.EventMessage:
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
			s.SendChannelMessage(fmt.Sprintf(":speech_balloon: %s: %s", who, event.Message))

		case client.EventUserJoin:
			playerName := fmt.Sprintf("User %d", event.Source)
			if event.UserInfo != nil {
				playerName = client.CString(event.UserInfo.Username[:])
				s.storeUser(uint32(event.Source), event.UserInfo.UniqueID, playerName)
			}
			slog.Info("player joined server",
				"name", name,
				"player", playerName,
				"source", event.Source,
			)
			s.sendEventEmbeddedMessage(ColorUserJoin, fmt.Sprintf("%s joined.", playerName))

		case client.EventUserLeave:
			playerName := s.username(uint32(event.Source))
			s.usersLock.Lock()
			delete(s.users, uint32(event.Source))
			s.usersLock.Unlock()
			slog.Info("player left RoR server",
				"name", name,
				"player", playerName,
				"source", event.Source,
			)
			s.sendEventEmbeddedMessage(ColorUserLeave, fmt.Sprintf("%s left.", playerName))

		case client.EventError:
			slog.Warn("unexpected server error", "name", name, "err", event.Err)
		}
	}

	slog.Debug("relay goroutine: event channel closed", "name", name)
}

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
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	ColorUserJoin  = 0x57F287 // green
	ColorUserLeave = 0xED4245 // red
	ColorDefault   = 0x5865F2
)

// SendChannelMessage sends a message to the server's Discord channel.
func (s *Server) SendChannelMessage(content string) {
	if _, err := s.session.ChannelMessageSend(s.ChannelID(), content); err != nil {
		s.Log().Error("failed to send discord message",
			"channel_id", s.ChannelID(),
			"err", err,
		)
	}
}

// SendChannelEmbeddedMessage sends an embedded message to the server's Discord channel.
func (s *Server) SendChannelEmbeddedMessage(embed *discordgo.MessageEmbed) {
	if _, err := s.session.ChannelMessageSendEmbed(s.ChannelID(), embed); err != nil {
		s.Log().Error("failed to send discord embedded message",
			"channel_id", s.ChannelID(),
			"err", err,
		)
	}
}

func (s *Server) sendEventEmbeddedMessage(color int, message string) {
	s.SendChannelEmbeddedMessage(&discordgo.MessageEmbed{
		Description: fmt.Sprintf("%s", message),
		Color:       color,
		Timestamp:   time.Now().Format(time.RFC3339),
	})
}

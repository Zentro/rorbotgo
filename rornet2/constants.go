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

package rornet2

// Version is the RoRnet protocol version string sent during handshake.
const Version = "RoRnet_2.45"

// MessageType represents the command field in a RoRnet packet header.
type MessageType uint32

const (
	MSG2_INVALID                 MessageType = 0
	MSG2_HELLO                   MessageType = 1025
	MSG2_FULL                    MessageType = 1026
	MSG2_WRONG_PW                MessageType = 1027
	MSG2_WRONG_VER               MessageType = 1028
	MSG2_BANNED                  MessageType = 1029
	MSG2_WELCOME                 MessageType = 1030
	MSG2_VERSION                 MessageType = 1031
	MSG2_SERVER_SETTINGS         MessageType = 1032
	MSG2_USER_INFO               MessageType = 1033
	MSG2_MASTERINFO              MessageType = 1034
	MSG2_NETQUALITY              MessageType = 1035
	MSG2_GAME_CMD                MessageType = 1036
	MSG2_USER_JOIN               MessageType = 1037
	MSG2_USER_LEAVE              MessageType = 1038
	MSG2_UTF_CHAT                MessageType = 1039
	MSG2_UTF_PRIVCHAT            MessageType = 1040
	MSG2_STREAM_REGISTER         MessageType = 1041
	MSG2_STREAM_REGISTER_RESULT  MessageType = 1042
	MSG2_STREAM_UNREGISTER       MessageType = 1043
	MSG2_STREAM_DATA             MessageType = 1044
	MSG2_STREAM_DATA_DISCARDABLE MessageType = 1045
	MSG2_NO_RANK                 MessageType = 1046 //!< client has no ranked status
	MSG2_WRONG_VER_LEGACY        MessageType = 1003
)

// UserAuth flags for the authstatus field in UserInfo.
type UserAuth int32

const (
	AUTH_NONE   UserAuth = 0
	AUTH_ADMIN  UserAuth = 1 << 0
	AUTH_RANKED UserAuth = 1 << 1
	AUTH_MOD    UserAuth = 1 << 2
	AUTH_BOT    UserAuth = 1 << 3
	AUTH_BANNED UserAuth = 1 << 4
)

// Netmask flags for vehicle state.
type Netmask uint32

const (
	NETMASK_HORN                      Netmask = 1 << 0
	NETMASK_LIGHTS                    Netmask = 1 << 1
	NETMASK_BRAKES                    Netmask = 1 << 2
	NETMASK_REVERSE                   Netmask = 1 << 3
	NETMASK_BEACONS                   Netmask = 1 << 4
	NETMASK_BLINK_LEFT                Netmask = 1 << 5
	NETMASK_BLINK_RIGHT               Netmask = 1 << 6
	NETMASK_BLINK_WARN                Netmask = 1 << 7
	NETMASK_CPARK                     Netmask = 1 << 8
	NETMASK_PBRAKE                    Netmask = 1 << 9
	NETMASK_TC_ACTIVE                 Netmask = 1 << 10
	NETMASK_ALB_ACTIVE                Netmask = 1 << 11
	NETMASK_ENGINE_CONT               Netmask = 1 << 12
	NETMASK_ENGINE_RUN                Netmask = 1 << 13
	NETMASK_ENGINE_MODE_AUTOMATIC     Netmask = 1 << 14
	NETMASK_ENGINE_MODE_SEMIAUTO      Netmask = 1 << 15
	NETMASK_ENGINE_MODE_MANUAL        Netmask = 1 << 16
	NETMASK_ENGINE_MODE_MANUAL_STICK  Netmask = 1 << 17
	NETMASK_ENGINE_MODE_MANUAL_RANGES Netmask = 1 << 18
)

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

package client

import (
	"bytes"
	"encoding/binary"
)

// Header is the 16-byte packet header prepended to every RoRnet message.
// Wire layout (little-endian): command(4) source(4) streamid(4) size(4).
type Header struct {
	Command  uint32
	Source   int32
	StreamID uint32
	Size     uint32
}

// UserInfo is sent by the client during the connection handshake (MSG2_USER_INFO)
// and returned by the server in the MSG2_WELCOME response.
type UserInfo struct {
	UniqueID       uint32
	AuthStatus     int32
	SlotNum        int32
	ColourNum      int32
	Username       [40]byte
	UserToken      [40]byte
	ServerPassword [40]byte
	Language       [10]byte
	ClientName     [10]byte
	ClientVersion  [25]byte
	ClientGUID     [40]byte
	SessionType    [10]byte
	SessionOptions [128]byte
}

// ServerInfo is sent by the server in response to MSG2_HELLO.
type ServerInfo struct {
	ProtocolVersion [20]byte
	Terrain         [128]byte
	ServerName      [128]byte
	HasPassword     uint8
	Info            [4096]byte
}

// StreamRegister is sent when a client registers a new stream.
type StreamRegister struct {
	Type           int32
	Status         int32
	OriginSourceID int32
	OriginStreamID int32
	Name           [128]byte
	Data           [128]byte
}

// StreamUnRegister is sent when a client unregisters a stream.
type StreamUnRegister struct {
	StreamID uint32
}

// VehicleState carries physics state for a vehicle stream.
type VehicleState struct {
	Time          int32
	EngineSpeed   float32
	EngineForce   float32
	EngineClutch  float32
	EngineGear    int32
	HydrodirState float32
	Brake         float32
	WheelSpeed    float32
	FlagMask      uint32
}

// CString returns the null-terminated content of a fixed byte array as a string.
func CString(b []byte) string {
	n := bytes.IndexByte(b, 0)
	if n < 0 {
		return string(b)
	}
	return string(b[:n])
}

// SetCString copies src into dst, zero-padding the remainder.
func SetCString(dst []byte, src string) {
	b := []byte(src)
	copy(dst, b)
	for i := len(b); i < len(dst); i++ {
		dst[i] = 0
	}
}

// MarshalBinary serializes a fixed-size struct to little-endian bytes.
func MarshalBinary(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary deserializes little-endian bytes into a fixed-size struct pointer.
func UnmarshalBinary(data []byte, v any) error {
	return binary.Read(bytes.NewReader(data), binary.LittleEndian, v)
}

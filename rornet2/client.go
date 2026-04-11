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

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"rorbotgo/system"
)

// EventKind identifies the type of event emitted by a Client.
type EventKind int

const (
	EventConnect    EventKind = iota // client successfully completed the handshake
	EventDisconnect                  // connection closed (clean or error)
	EventMessage                     // UTF-8 chat message received
	EventUserJoin                    // a player joined the server
	EventUserLeave                   // a player left the server
	EventUserSync                    // existing player info received on initial connect (track, don't announce)
	EventError                       // a non-fatal error occurred
)

// Event is emitted on the Client.Events channel for every notable occurrence.
type Event struct {
	Kind     EventKind
	Source   int32
	Message  string    // EventMessage: chat text
	UserInfo *UserInfo // EventUserJoin: player info
	Err      error     // EventError / EventDisconnect
}

// Client manages a single TCP connection to a server and runs the receive
// loop in its own goroutine. Each server in the bot gets one Client.
type Client struct {
	host     string
	port     int
	username string
	password string
	language string
	token	 string

	mu        sync.Mutex
	conn      net.Conn
	connected bool
	uniqueID  uint32

	// Events receives all events from this client. The caller must drain it;
	// a full channel will block the receive loop.
	Events chan Event

	stop chan struct{}
}

// NewClient creates a new, unconnected Client.
func NewClient(host string, port int, username, password, language string, token string) *Client {
	return &Client{
		host:     host,
		port:     port,
		username: username,
		password: password,
		language: language,
		token: 	  token,
		Events:   make(chan Event, 64),
		stop:     make(chan struct{}),
	}
}

// Connect performs the RoRnet handshake and, on success, starts the receive
// loop goroutine. It returns once the handshake is complete.
func (c *Client) Connect() error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", c.host, c.port), 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()

	slog.Info("connected to server", "host", c.host, "port", c.port)

	if err := c.handshake(); err != nil {
		conn.Close()
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
		return err
	}

	c.emit(Event{Kind: EventConnect})
	go c.receiveLoop()
	go c.keepalive()
	return nil
}

// Disconnect sends MSG2_USER_LEAVE, stops the receive loop, and closes the
// connection. Safe to call more than once.
func (c *Client) Disconnect() {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return
	}
	c.connected = false
	conn := c.conn
	c.mu.Unlock()

	_ = c.sendPacket(uint32(MSG2_USER_LEAVE), c.uniqueID, 0, nil)
	close(c.stop)
	conn.Close()
	slog.Info("disconnected from server", "host", c.host, "port", c.port)
}

// IsConnected reports whether the client currently has an active connection.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// SendChat sends a UTF-8 chat message to the server.
func (c *Client) SendChat(message string) error {
	return c.sendPacket(uint32(MSG2_UTF_CHAT), int32(c.uniqueID), 0, []byte(message))
}

// --------------------------------------------------------------------------
// internal helpers
// --------------------------------------------------------------------------

func (c *Client) handshake() error {
	// Send MSG2_HELLO with the protocol version string.
	if err := c.sendPacket(uint32(MSG2_HELLO), 0, 0, []byte(Version)); err != nil {
		return fmt.Errorf("send hello: %w", err)
	}

	// Expect MSG2_HELLO back with ServerInfo payload.
	hdr, payload, err := c.readPacket()
	if err != nil {
		return fmt.Errorf("read server hello: %w", err)
	}
	if MessageType(hdr.Command) == MSG2_WRONG_VER {
		return fmt.Errorf("server rejected protocol version")
	}
	if MessageType(hdr.Command) != MSG2_HELLO {
		return fmt.Errorf("expected MSG2_HELLO, got %d", hdr.Command)
	}

	var si ServerInfo
	if err := UnmarshalBinary(payload, &si); err != nil {
		return fmt.Errorf("unmarshal server info: %w", err)
	}
	slog.Info("server info",
		"name", CString(si.ServerName[:]),
		"terrain", CString(si.Terrain[:]),
		"protocol", CString(si.ProtocolVersion[:]),
	)

	// 3. Send MSG2_USER_INFO.
	var ui UserInfo
	SetCString(ui.Username[:], c.username)
	SetCString(ui.ServerPassword[:], c.password)
	SetCString(ui.Language[:], c.language)
	SetCString(ui.UserToken[:], c.token)
	SetCString(ui.ClientName[:], "RoRBot")
	SetCString(ui.ClientVersion[:], system.Version)
	SetCString(ui.SessionType[:], "normal")
	ui.SlotNum = -1

	uiBytes, err := MarshalBinary(&ui)
	if err != nil {
		return fmt.Errorf("marshal user info: %w", err)
	}
	if err := c.sendPacket(uint32(MSG2_USER_INFO), 0, 0, uiBytes); err != nil {
		return fmt.Errorf("send user info: %w", err)
	}

	// 4. Expect MSG2_WELCOME (or an error code).
	hdr, payload, err = c.readPacket()
	if err != nil {
		return fmt.Errorf("read welcome: %w", err)
	}
	switch MessageType(hdr.Command) {
	case MSG2_WELCOME:
		var assigned UserInfo
		if err := UnmarshalBinary(payload, &assigned); err != nil {
			return fmt.Errorf("unmarshal welcome user info: %w", err)
		}
		c.mu.Lock()
		c.uniqueID = assigned.UniqueID
		c.mu.Unlock()
		slog.Info("joined server",
			"unique_id", assigned.UniqueID,
			"username", CString(assigned.Username[:]),
		)
	case MSG2_FULL:
		return fmt.Errorf("server is full")
	case MSG2_BANNED:
		return fmt.Errorf("banned from server")
	case MSG2_WRONG_PW:
		return fmt.Errorf("wrong password")
	case MSG2_NO_RANK:
		return fmt.Errorf("server requires a user token (no ranked status)")
	default:
		return fmt.Errorf("unexpected response %d", hdr.Command)
	}

	return nil
}

func (c *Client) receiveLoop() {
	defer func() {
		c.mu.Lock()
		wasConnected := c.connected
		c.connected = false
		c.mu.Unlock()
		if wasConnected {
			c.emit(Event{Kind: EventDisconnect})
		}
	}()

	for {
		select {
		case <-c.stop:
			return
		default:
		}

		hdr, payload, err := c.readPacket()
		if err != nil {
			select {
			case <-c.stop:
				return
			default:
			}
			// Mark disconnected before emitting so the deferred cleanup
			// does not fire a second EventDisconnect.
			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()
			c.emit(Event{Kind: EventDisconnect, Err: err})
			return
		}

		c.processPacket(hdr, payload)
	}
}

// keepalive sends MSG2_NETQUALITY every 5 seconds so the server's waitIO()
// timeout never fires. The server drops silent clients regardless of whether
// they have an active stream, so any well-formed packet is sufficient.
func (c *Client) keepalive() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			// Payload: single uint32 quality value (100 = perfect).
			var payload [4]byte
			binary.LittleEndian.PutUint32(payload[:], 100)
			if err := c.sendPacket(uint32(MSG2_NETQUALITY), int32(c.uniqueID), 0, payload[:]); err != nil {
				slog.Debug("keepalive send failed", "host", c.host, "err", err)
				return
			}
			slog.Debug("keepalive sent", "host", c.host)
		}
	}
}

func (c *Client) processPacket(hdr Header, payload []byte) {
	slog.Debug("packet", "command", hdr.Command, "source", hdr.Source, "size", hdr.Size)
	switch MessageType(hdr.Command) {
	case MSG2_UTF_CHAT:
		source := hdr.Source
		// Sources > 100000 are system messages.
		if source > 100000 {
			source = -1
		}
		// RoR sends chat as a null-terminated C string; strip the terminator.
		msg := CString(payload)
		slog.Debug("received chat packet", "source", source, "len", len(payload), "message", msg)
		c.emit(Event{
			Kind:    EventMessage,
			Source:  source,
			Message: msg,
		})

	case MSG2_USER_JOIN:
		var ui UserInfo
		if err := UnmarshalBinary(payload, &ui); err != nil {
			slog.Warn("failed to parse UserInfo on join", "err", err)
			return
		}
		c.emit(Event{Kind: EventUserJoin, Source: hdr.Source, UserInfo: &ui})

	case MSG2_USER_LEAVE:
		c.emit(Event{Kind: EventUserLeave, Source: hdr.Source})

	case MSG2_USER_INFO:
		// Sent by the server for each user already on the server when we join.
		var ui UserInfo
		if err := UnmarshalBinary(payload, &ui); err != nil {
			slog.Warn("failed to parse UserInfo sync packet", "err", err)
			return
		}
		c.emit(Event{Kind: EventUserSync, Source: hdr.Source, UserInfo: &ui})

	// Explicitly ignored packet types.
	case MSG2_STREAM_DATA,
		MSG2_STREAM_DATA_DISCARDABLE,
		MSG2_STREAM_REGISTER,
		MSG2_STREAM_REGISTER_RESULT,
		MSG2_STREAM_UNREGISTER,
		MSG2_NETQUALITY,
		MSG2_GAME_CMD:
		// no-op

	default:
		slog.Debug("unhandled packet", "command", hdr.Command, "source", hdr.Source)
	}
}

// readPacket reads one complete packet (header + payload) from the wire.
func (c *Client) readPacket() (Header, []byte, error) {
	var hdr Header
	if err := binary.Read(c.conn, binary.LittleEndian, &hdr); err != nil {
		return hdr, nil, fmt.Errorf("read header: %w", err)
	}

	if hdr.Size == 0 {
		return hdr, nil, nil
	}

	payload := make([]byte, hdr.Size)
	if _, err := io.ReadFull(c.conn, payload); err != nil {
		return hdr, nil, fmt.Errorf("read payload (%d bytes): %w", hdr.Size, err)
	}
	return hdr, payload, nil
}

// sendPacket writes a header + optional payload to the wire.
func (c *Client) sendPacket(command uint32, source any, streamID uint32, data []byte) error {
	var src uint32
	switch v := source.(type) {
	case int32:
		src = uint32(v)
	case uint32:
		src = v
	}

	buf := make([]byte, 16+len(data))
	binary.LittleEndian.PutUint32(buf[0:], command)
	binary.LittleEndian.PutUint32(buf[4:], src)
	binary.LittleEndian.PutUint32(buf[8:], streamID)
	binary.LittleEndian.PutUint32(buf[12:], uint32(len(data)))
	copy(buf[16:], data)

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("not connected")
	}
	_, err := conn.Write(buf)
	return err
}

func (c *Client) emit(e Event) {
	select {
	case c.Events <- e:
	default:
		slog.Warn("event channel full, dropping event", "kind", e.Kind)
	}
}

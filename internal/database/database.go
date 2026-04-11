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

package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"rorbotgo/internal/models"
)

// Database wraps a SQLite connection and provides typed access for bot data.
type Database struct {
	db *sql.DB
}

// Initialize opens (or creates) the SQLite database at path and runs migrations.
func Initialize(path string) (*Database, error) {
	slog.Info("initializing database", "path", path)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	d := &Database{db: sqlDB}
	slog.Debug("running database migrations")
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	slog.Info("database ready", "path", path)
	return d, nil
}

// Close closes the underlying database connection.
func (d *Database) Close() error {
	slog.Debug("closing database connection")
	return d.db.Close()
}

func (d *Database) migrate() error {
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS servers (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT    NOT NULL,
			guild_id   INTEGER NOT NULL,
			channel_id INTEGER NOT NULL UNIQUE,
			host       TEXT    NOT NULL,
			port       INTEGER NOT NULL,
			password   TEXT    NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		slog.Error("migration failed", "err", err)
		return err
	}
	slog.Debug("migrations applied")
	return nil
}

// CreateServer inserts a new server record and returns the created model.
func (d *Database) CreateServer(name string, guildID, channelID int64, host string, port int, password string) (*models.Server, error) {
	slog.Debug("CreateServer",
		"name", name,
		"guild_id", guildID,
		"channel_id", channelID,
		"host", host,
		"port", port,
	)

	res, err := d.db.Exec(
		`INSERT INTO servers (name, guild_id, channel_id, host, port, password) VALUES (?, ?, ?, ?, ?, ?)`,
		name, guildID, channelID, host, port, password,
	)
	if err != nil {
		slog.Error("CreateServer failed", "name", name, "err", err)
		return nil, err
	}

	id, _ := res.LastInsertId()
	slog.Debug("CreateServer success", "id", id, "name", name)
	return d.GetServerByID(id)
}

// GetServerByID returns the server with the given primary key.
func (d *Database) GetServerByID(id int64) (*models.Server, error) {
	slog.Debug("GetServerByID", "id", id)
	row := d.db.QueryRow(
		`SELECT id, name, guild_id, channel_id, host, port, password, created_at FROM servers WHERE id = ?`, id)
	srv, err := scanServer(row)
	if err != nil {
		slog.Debug("GetServerByID not found", "id", id, "err", err)
	}
	return srv, err
}

// GetServerByChannel returns the server bound to a Discord channel.
func (d *Database) GetServerByChannel(channelID int64) (*models.Server, error) {
	slog.Debug("GetServerByChannel", "channel_id", channelID)
	row := d.db.QueryRow(
		`SELECT id, name, guild_id, channel_id, host, port, password, created_at FROM servers WHERE channel_id = ?`, channelID)
	srv, err := scanServer(row)
	if err != nil {
		slog.Debug("GetServerByChannel not found", "channel_id", channelID, "err", err)
	}
	return srv, err
}

// GetServerByName returns the server with the given name within a guild.
func (d *Database) GetServerByName(guildID int64, name string) (*models.Server, error) {
	slog.Debug("GetServerByName", "guild_id", guildID, "name", name)
	row := d.db.QueryRow(
		`SELECT id, name, guild_id, channel_id, host, port, password, created_at FROM servers WHERE guild_id = ? AND name = ?`,
		guildID, name)
	srv, err := scanServer(row)
	if err != nil {
		slog.Debug("GetServerByName not found", "guild_id", guildID, "name", name, "err", err)
	}
	return srv, err
}

// ListServers returns all servers registered in a guild, ordered by name.
func (d *Database) ListServers(guildID int64) ([]*models.Server, error) {
	slog.Debug("ListServers", "guild_id", guildID)

	rows, err := d.db.Query(
		`SELECT id, name, guild_id, channel_id, host, port, password, created_at FROM servers WHERE guild_id = ? ORDER BY name`,
		guildID)
	if err != nil {
		slog.Error("ListServers query failed", "guild_id", guildID, "err", err)
		return nil, err
	}
	defer rows.Close()

	var servers []*models.Server
	for rows.Next() {
		s, err := scanServer(rows)
		if err != nil {
			slog.Error("ListServers scan failed", "guild_id", guildID, "err", err)
			return nil, err
		}
		servers = append(servers, s)
	}
	if err := rows.Err(); err != nil {
		slog.Error("ListServers iteration error", "guild_id", guildID, "err", err)
		return nil, err
	}

	slog.Debug("ListServers complete", "guild_id", guildID, "count", len(servers))
	return servers, nil
}

// DeleteServer removes a server record by ID.
func (d *Database) DeleteServer(id int64) error {
	slog.Debug("DeleteServer", "id", id)
	_, err := d.db.Exec(`DELETE FROM servers WHERE id = ?`, id)
	if err != nil {
		slog.Error("DeleteServer failed", "id", id, "err", err)
		return err
	}
	slog.Debug("DeleteServer success", "id", id)
	return nil
}

// scanner abstracts *sql.Row and *sql.Rows so scanServer works for both.
type scanner interface {
	Scan(dest ...any) error
}

func scanServer(s scanner) (*models.Server, error) {
	var sv models.Server
	var createdAt string
	if err := s.Scan(&sv.ID, &sv.Name, &sv.GuildID, &sv.ChannelID, &sv.Host, &sv.Port, &sv.Password, &createdAt); err != nil {
		return nil, err
	}
	sv.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &sv, nil
}

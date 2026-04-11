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

package config

import (
	"os"
	"sync"

	"gopkg.in/yaml.v2"
)

const DefaultLocation = "/etc/rorbotgo/config.yml"

var (
	mu      sync.RWMutex
	_config *Configuration
)

type Configuration struct {
	path string

	LogDirectory string `default:"/var/log/rorbotgo" yaml:"log_directory"`

	RootDirectory string `default:"/var/lib/rorbotgo" yaml:"root_directory"`

	// Should run in debug mode or production mode. This value is ignored
	// if the debug flag is passed in command line arguments.
	Debug bool `default:"true" yaml:"debug"`

	Discord  DiscordConfiguration `yaml:"discord"`
	Database DbConfiguration      `yaml:"db"`
	Bot	 BotConfiguration     `yaml:"bot"`
}

type DiscordConfiguration struct {
	Token string `yaml:"token"`
}

type DbConfiguration struct {
	Path string `yaml:"path"`
}

type BotConfiguration struct {
	Username string `yaml:"username"`
	Language string `yaml:"language"`
	Token    string `yaml:"token"`
}

func FromFile(path string) error {
	f, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	c := new(Configuration)
	c.path = path

	if err := yaml.Unmarshal(f, c); err != nil {
		return err
	}

	Set(c)
	return nil
}

func Set(c *Configuration) {
	mu.Lock()
	_config = c
	mu.Unlock()
}

func Get() *Configuration {
	mu.RLock()
	c := *_config
	mu.RUnlock()
	return &c
}

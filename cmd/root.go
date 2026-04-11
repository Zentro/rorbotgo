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

package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/mitchellh/colorstring"
	"github.com/spf13/cobra"

	"rorbotgo/bot"
	"rorbotgo/config"
	"rorbotgo/internal/database"
	"rorbotgo/system"
)

var rootCmd = &cobra.Command{
	Use:   "rorbotgo",
	Short: "RoRBot(Go) is a Discord bot for managing and interacting with Rigs of Rods servers",
	PreRun: func(cmd *cobra.Command, args []string) {
		initConfig()
		initLogging()
	},
	RunE: rootCmdRun,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version and quit",
	Run: func(cmd *cobra.Command, _ []string) {
		fmt.Printf("v%s\nCopyright (c) 2025 Rafael Galvan and contributors\n", system.Version)
	},
}

var (
	debug      bool
	configPath string
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "run in debug mode")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", config.DefaultLocation, "set the location for the config file")
	rootCmd.AddCommand(versionCmd)
}

func rootCmdRun(cmd *cobra.Command, _ []string) error {
	printLogo()

	cfg := config.Get()

	db, err := database.Initialize(cfg.Database.Path)
	if err != nil {
		slog.Error("could not initialize database", "error", err)
		return err
	}
	defer db.Close()

	b, err := bot.New(cfg, db)
	if err != nil {
		slog.Error("could not create bot", "error", err)
		return err
	}

	return b.Start()
}

func initConfig() {
	if err := config.FromFile(configPath); err != nil {
		exitWithConfigurationError(err)
	}

	if debug {
		cfg := config.Get()
		cfg.Debug = true
		config.Set(cfg)
	}
}

func initLogging() {
	cfg := config.Get()

	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}

	writers := []io.Writer{os.Stderr}

	if cfg.LogDirectory != "" {
		if err := os.MkdirAll(cfg.LogDirectory, 0o755); err == nil {
			logPath := filepath.Join(cfg.LogDirectory, "rorbotgo.log")
			f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err == nil {
				writers = append(writers, f)
			}
		}
	}

	handler := slog.NewTextHandler(io.MultiWriter(writers...), &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}

func printLogo() {
	fmt.Printf(colorstring.Color(`[blue]
____   __  ____  ____   __  ____  ___   __
(  _ \ /  \(  _ \(  _ \ /  \(_  _)/ __) /  \
)   /(  O ))   / ) _ ((  O ) )( ( (_ \(  O )
(__\_) \__/(__\_)(____/ \__/ (__) \___/ \__/   [reset]

Copyright (c) 2025 Rafael Galvan and contributors.

RoRBotGo (hayai) [Version %s]

[bold]Use of this source code is governed by the GPLv3 license.
The license can be found in the LICENSE file.
[reset]

Learn more at https://www.rigsofrods.org

`), system.Version)
}

func exitWithConfigurationError(err error) {
	fmt.Printf(colorstring.Color(`
[_red_][white][bold]ERROR! %s[reset]

RoRBot(Go) was not able to locate the configuration file at %s.
Please check the path and try again.
	`), err, configPath)
	os.Exit(1)
}

/*
 * Bamboo - A Vietnamese Input method editor
 * Copyright (C) 2018 Luong Thanh Lam <ltlam93@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"ibus-bamboo/config"
)

// An introspector tells which application currently has focus, so the
// engine can apply the per-application input mode.
const (
	IntrospectorAuto    = "auto"
	IntrospectorX11     = "x11"
	IntrospectorGnome   = "gnome"
	IntrospectorWayland = "wayland"
	IntrospectorCommand = "command"
)

// introspectCommandTimeout bounds the user command: it runs on every
// FocusIn, so a hung command must not stall the engine.
const introspectCommandTimeout = 500 * time.Millisecond

// Resolved once at startup by setupIntrospector, read-only afterwards.
var activeIntrospector = IntrospectorX11
var introspectorCommand string

func hasGnome(env string) bool {
	return strings.Contains(strings.ToLower(os.Getenv(env)), "gnome")
}

// guessIntrospector picks an introspector from the running session.
// GNOME is checked first: GNOME on Wayland does not implement the wlr
// foreign toplevel protocol used by the Wayland introspector.
func guessIntrospector() string {
	if hasGnome("XDG_CURRENT_DESKTOP") || hasGnome("DESKTOP_SESSION") || hasGnome("GDMSESSION") {
		return IntrospectorGnome
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return IntrospectorWayland
	}
	return IntrospectorX11
}

// resolveIntrospector maps the configured name to the introspector to use.
// An unknown name, or "command" without a command, falls back to "auto"
// rather than failing: a typo in the config must not disable typing.
func resolveIntrospector(name, command string) string {
	switch name := strings.ToLower(strings.TrimSpace(name)); name {
	case "", IntrospectorAuto:
		return guessIntrospector()
	case IntrospectorX11, IntrospectorGnome, IntrospectorWayland:
		return name
	case IntrospectorCommand:
		if strings.TrimSpace(command) != "" {
			return name
		}
		log.Printf("Introspector %q needs IntrospectorCommand; using auto", name)
	default:
		log.Printf("Unknown introspector %q; using auto", name)
	}
	return guessIntrospector()
}

// setupIntrospector reads the introspector settings from the config and
// starts any background work the chosen introspector needs.
func setupIntrospector(cfg *config.Config) {
	activeIntrospector = resolveIntrospector(cfg.Introspector, cfg.IntrospectorCommand)
	introspectorCommand = cfg.IntrospectorCommand
	log.Printf("Using introspector: %s", activeIntrospector)
	if activeIntrospector == IntrospectorWayland {
		go func() {
			if err := wlGetFocusWindowClass(); err != nil {
				log.Printf("Wayland introspector: %s", err)
			}
		}()
	}
}

// cmdGetFocusWindowClass runs the user command through the shell and
// returns the first line it prints, e.g. an app_id or a WM_CLASS.
func cmdGetFocusWindowClass(command string) string {
	ctx, cancel := context.WithTimeout(context.Background(), introspectCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	// Killing sh does not kill its children, which keep stdout open;
	// stop waiting for them shortly after the deadline.
	cmd.WaitDelay = 100 * time.Millisecond
	out, err := cmd.Output()
	if err != nil {
		log.Printf("Introspector command %q failed: %s", command, err)
		return ""
	}
	line, _, _ := strings.Cut(string(out), "\n")
	return strings.TrimSpace(line)
}

// getFocusWindowClass returns the class of the focused window, falling
// back to X11 when the active introspector cannot tell.
func getFocusWindowClass() string {
	var wmClass string
	switch activeIntrospector {
	case IntrospectorGnome:
		wmClass, _ = gnomeGetFocusWindowClass()
	case IntrospectorWayland:
		wmClass = getWlAppId()
	case IntrospectorCommand:
		wmClass = cmdGetFocusWindowClass(introspectorCommand)
	}
	if wmClass == "" {
		wmClass = x11GetFocusWindowClass()
	}
	return strings.Replace(wmClass, "\"", "", -1)
}

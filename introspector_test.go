package main

import (
	"strings"
	"testing"
	"time"
)

func clearSessionEnv(t *testing.T) {
	for _, env := range []string{"XDG_CURRENT_DESKTOP", "DESKTOP_SESSION", "GDMSESSION", "WAYLAND_DISPLAY"} {
		t.Setenv(env, "")
	}
}

func TestResolveIntrospector(t *testing.T) {
	clearSessionEnv(t)
	cases := []struct {
		name, command, want string
	}{
		{"", "", IntrospectorX11},
		{"auto", "", IntrospectorX11},
		{"gnome", "", IntrospectorGnome},
		{" Wayland ", "", IntrospectorWayland},
		{"x11", "", IntrospectorX11},
		{"command", "echo foo", IntrospectorCommand},
		// Misconfigurations fall back to auto instead of failing.
		{"command", "  ", IntrospectorX11},
		{"no-such-thing", "", IntrospectorX11},
	}
	for _, c := range cases {
		if got := resolveIntrospector(c.name, c.command); got != c.want {
			t.Errorf("resolveIntrospector(%q, %q) = %q, want %q", c.name, c.command, got, c.want)
		}
	}
}

func TestGuessIntrospector(t *testing.T) {
	clearSessionEnv(t)
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	if got := guessIntrospector(); got != IntrospectorWayland {
		t.Errorf("wayland session: got %q", got)
	}
	// GNOME wins over Wayland: mutter lacks the wlr toplevel protocol.
	t.Setenv("XDG_CURRENT_DESKTOP", "ubuntu:GNOME")
	if got := guessIntrospector(); got != IntrospectorGnome {
		t.Errorf("gnome wayland session: got %q", got)
	}
}

func TestCmdGetFocusWindowClass(t *testing.T) {
	cases := []struct {
		command, want string
	}{
		{`printf '  firefox \n'`, "firefox"},
		{`printf 'org.gnome.Nautilus\nsecond line\n'`, "org.gnome.Nautilus"},
		{`echo kitty | tr a-z A-Z`, "KITTY"},
		{`exit 1`, ""},
		{`/nonexistent/introspector`, ""},
	}
	for _, c := range cases {
		if got := cmdGetFocusWindowClass(c.command); got != c.want {
			t.Errorf("cmdGetFocusWindowClass(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}

func TestCmdGetFocusWindowClassTimeout(t *testing.T) {
	// The second form leaves a grandchild holding stdout after sh is killed.
	for _, command := range []string{"exec sleep 5", "sleep 5; echo late"} {
		start := time.Now()
		got := cmdGetFocusWindowClass(command)
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("%q blocked for %s", command, elapsed)
		}
		if got != "" {
			t.Errorf("timed out %q returned %q", command, got)
		}
	}
}

func TestGetFocusWindowClassCommand(t *testing.T) {
	saved, savedCmd := activeIntrospector, introspectorCommand
	t.Cleanup(func() { activeIntrospector, introspectorCommand = saved, savedCmd })
	activeIntrospector = IntrospectorCommand
	introspectorCommand = `echo '"quoted"'`
	if got := getFocusWindowClass(); strings.Contains(got, `"`) || got != "quoted" {
		t.Errorf("getFocusWindowClass() = %q, want %q", got, "quoted")
	}
}

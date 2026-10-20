package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/next/internal/adapter/registry"
)

// TestMain keeps every test of the package off the machine's config directory, the one that forgets ownConfigDir included.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "wtm-next-home")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", home)
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}

// A test that set $HOME alone wrote the runner's real repos.json: on Linux $XDG_CONFIG_HOME wins.
func TestNoTestReachesTheMachinesConfigDirectory(t *testing.T) {
	assert.True(t, strings.HasPrefix(registry.Path(), os.TempDir()), registry.Path())
}

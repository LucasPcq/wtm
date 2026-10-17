package proxyport

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAProxySwitchedOffAnnouncesNoPort(t *testing.T) {
	assert.Zero(t, Port{Plist: plistIn(t)}.Public(t.Context(), 0))
}

func TestAnInstalledRedirectionAnnouncesThePrivilegedPort(t *testing.T) {
	plist := plistIn(t)
	require.NoError(t, os.WriteFile(plist, []byte("<plist/>"), 0o644))

	assert.Equal(t, 80, Port{Plist: plist}.Public(t.Context(), 11080))
}

func TestWithoutARedirectionTheConfiguredPortIsAnnounced(t *testing.T) {
	assert.Equal(t, 11080, Port{Plist: plistIn(t)}.Public(t.Context(), 11080))
}

func TestTheRedirectionIsLaunchdsAgentInTheHomeDirectory(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(home, "Library", "LaunchAgents", "dev.wtm.proxy.plist"), PlistPath())
}

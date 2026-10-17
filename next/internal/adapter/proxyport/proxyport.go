// Package proxyport says which port a named URL announces on this machine: the proxy's own, or 80 once the redirection is installed.
package proxyport

import (
	"context"
	"os"
	"path/filepath"
)

const (
	privilegedPort = 80
	agentDir       = "Library/LaunchAgents"
	plistName      = "dev.wtm.proxy.plist"
)

// Port finds the port a named URL announces from whether the launchd agent
// that redirects port 80 to the proxy is installed.
type Port struct {
	Plist string // the agent's plist; empty never finds the redirection
}

// PlistPath returns where the redirection's launchd agent is installed, in the
// user's LaunchAgents, or "" when the home directory is unknown.
func PlistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, agentDir, plistName)
}

// Public returns the port a named URL announces: 0 when the proxy is off
// (configured is 0), 80 when the redirection's agent is installed, configured
// otherwise.
func (p Port) Public(_ context.Context, configured int) int {
	if configured == 0 {
		return 0
	}

	if p.Plist == "" {
		return configured
	}
	if _, err := os.Stat(p.Plist); err != nil {
		return configured
	}
	return privilegedPort
}

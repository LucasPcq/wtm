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

type Port struct {
	Plist string
}

func PlistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, agentDir, plistName)
}

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

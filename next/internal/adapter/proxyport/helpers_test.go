package proxyport

import (
	"path/filepath"
	"testing"
)

func plistIn(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "dev.wtm.proxy.plist")
}

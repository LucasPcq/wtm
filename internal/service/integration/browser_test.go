package integration

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestOpenerFor(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{domain.GOOSDarwin, domain.OpenerDarwin},
		{domain.GOOSWindows, domain.OpenerWindows},
		{"linux", domain.OpenerUnix},
		{"freebsd", domain.OpenerUnix},
	}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			if got := openerFor(tt.goos).Name; got != tt.want {
				t.Errorf("openerFor(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}

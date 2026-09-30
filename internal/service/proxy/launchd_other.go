//go:build !darwin

package proxy

import (
	"net"

	"github.com/LucasPcq/wtm/internal/domain"
)

func LaunchdListeners(string) ([]net.Listener, error) {
	return nil, domain.ErrProxyRedirectUnsupported
}

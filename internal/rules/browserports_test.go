package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// The default is the whole point of the list: a proxy nobody can reach from a
// browser serves nothing, however healthy it looks.
func TestDefaultProxyPortIsReachableFromABrowser(t *testing.T) {
	if IsBrowserBlockedPort(domain.ProxyDefaultPort) {
		t.Errorf("the default proxy port %d is on the browsers' blocked list", domain.ProxyDefaultPort)
	}
}

// 10080 was the default until it turned out every named URL answered
// ERR_UNSAFE_PORT. It stays in the list so a config still carrying it is
// stepped over rather than served.
func TestKnownBlockedPortsAreRecognised(t *testing.T) {
	for _, port := range []int{10080, 6000, 6697, 2049, 22, 25} {
		if !IsBrowserBlockedPort(port) {
			t.Errorf("port %d is blocked by browsers but not recognised", port)
		}
	}
}

func TestOrdinaryDevPortsAreNotBlocked(t *testing.T) {
	for _, port := range []int{3000, 4001, 5173, 8080, 11080, 11081} {
		if IsBrowserBlockedPort(port) {
			t.Errorf("port %d is a normal dev port but was reported blocked", port)
		}
	}
}

// The scan span starts at the configured port, so a blocked default must not
// strand it: the whole window has to hold a usable port.
func TestTheScanWindowHoldsAUsablePort(t *testing.T) {
	usable := 0
	for port := domain.ProxyDefaultPort; port < domain.ProxyDefaultPort+domain.ProxyPortScanSpan; port++ {
		if !IsBrowserBlockedPort(port) {
			usable++
		}
	}
	if usable == 0 {
		t.Fatal("every port in the scan window is blocked by browsers")
	}
}

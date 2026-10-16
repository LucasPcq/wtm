package runenv

import (
	"fmt"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
)

const (
	PrivilegedProxyPort = 80
	defaultProxyPort    = 11080
	portMin             = 1
	portMax             = 65535
)

func ProxyPort(global project.Global) int {
	if global.Proxy.Enabled != nil && !*global.Proxy.Enabled {
		return 0
	}
	if global.Proxy.Port > 0 {
		return global.Proxy.Port
	}
	return defaultProxyPort
}

func validateProxy(proxy project.Proxy) error {
	if proxy.Port == 0 || (proxy.Port >= portMin && proxy.Port <= portMax) {
		return nil
	}
	return fmt.Errorf("invalid [proxy] port %d in config.toml — use %d-%d, or leave it out for %d", proxy.Port, portMin, portMax, defaultProxyPort)
}

package runenv

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/stretchr/testify/assert"
)

func TestTheProxyPortIsTheConfiguredOneElseTheDefaultUnlessSwitchedOff(t *testing.T) {
	on, off := true, false
	cases := []struct {
		name   string
		global project.Global
		want   int
	}{
		{"absent: the default", project.Global{}, defaultProxyPort},
		{"a chosen port", project.Global{Proxy: project.Proxy{Port: 5000}}, 5000},
		{"switched off", project.Global{Proxy: project.Proxy{Enabled: &off, Port: 5000}}, 0},
		{"switched on without a port", project.Global{Proxy: project.Proxy{Enabled: &on}}, defaultProxyPort},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ProxyPort(tc.global))
		})
	}
}

func TestAProxyPortASocketCannotBindIsRefused(t *testing.T) {
	for port, valid := range map[int]bool{0: true, 1: true, 11080: true, 65535: true, -1: false, 65536: false, 70000: false} {
		err := validateProxy(project.Proxy{Port: port})

		assert.Equal(t, valid, err == nil, "port %d: %v", port, err)
	}
	assert.EqualError(t, validateProxy(project.Proxy{Port: 70000}), "invalid [proxy] port 70000 in config.toml — use 1-65535, or leave it out for 11080")
}

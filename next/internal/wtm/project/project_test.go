package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTheProxyPortIsTheConfiguredOneElseTheDefaultUnlessSwitchedOff(t *testing.T) {
	cases := []struct {
		name   string
		global Global
		want   int
	}{
		{"absent: the default", Global{}, defaultProxyPort},
		{"a chosen port", Global{Proxy: Proxy{Port: 5000}}, 5000},
		{"switched off", Global{Proxy: Proxy{Enabled: new(false), Port: 5000}}, 0},
		{"switched on without a port", Global{Proxy: Proxy{Enabled: new(true)}}, defaultProxyPort},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.global.ProxyPort())
		})
	}
}

func TestAProxyPortASocketCannotBindIsAProblem(t *testing.T) {
	for port, valid := range map[int]bool{0: true, 1: true, 11080: true, 65535: true, -1: false, 65536: false, 70000: false} {
		err := Proxy{Port: port}.Problem()

		assert.Equal(t, valid, err == nil, "port %d: %v", port, err)
	}
	assert.EqualError(t, Proxy{Port: 70000}.Problem(), "invalid [proxy] port 70000 in config.toml — use 1-65535, or leave it out for 11080")
}

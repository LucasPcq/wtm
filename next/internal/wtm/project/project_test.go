package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTheProxyPortIsTheConfiguredOneElseTheDefaultUnlessSwitchedOff(t *testing.T) {
	on, off := true, false
	cases := []struct {
		name   string
		global Global
		want   int
	}{
		{"absent: the default", Global{}, defaultProxyPort},
		{"a chosen port", Global{Proxy: Proxy{Port: 5000}}, 5000},
		{"switched off", Global{Proxy: Proxy{Enabled: &off, Port: 5000}}, 0},
		{"switched on without a port", Global{Proxy: Proxy{Enabled: &on}}, defaultProxyPort},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.global.ProxyPort())
		})
	}
}

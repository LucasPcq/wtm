package rules

import "testing"

func TestPublicPort(t *testing.T) {
	tests := []struct {
		name     string
		bindPort int
		probed   int
		declared bool
		daemonUp bool
		want     int
	}{
		{name: "proxy éteint", want: 0},
		{name: "daemon up, sonde concordante : le 80", bindPort: 4000, probed: 4000, daemonUp: true, want: 80},
		{name: "daemon up, rien derrière le 80", bindPort: 4000, daemonUp: true, want: 4000},
		{name: "daemon up, redirection périmée", bindPort: 4001, probed: 4000, daemonUp: true, want: 4001},
		{name: "daemon up : la sonde prime sur le déclaré", bindPort: 4000, declared: true, daemonUp: true, want: 4000},
		{name: "hors daemon, redirection déclarée", bindPort: 4000, declared: true, want: 80},
		{name: "hors daemon, rien de déclaré", bindPort: 4000, want: 4000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PublicPort(PublicPortParams{BindPort: tt.bindPort, Probed: tt.probed, Declared: tt.declared, DaemonUp: tt.daemonUp})
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

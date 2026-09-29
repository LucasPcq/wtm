package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestRouteHost(t *testing.T) {
	published := func(name, host string) domain.JobConfig {
		return domain.JobConfig{Name: name, Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT", Host: host}}
	}

	tests := []struct {
		name     string
		job      domain.JobConfig
		worktree string
		project  string
		want     string
	}{
		{"nom du job par défaut", published("app1-web", ""), "feat-auth", "myapp", "app1-web.feat-auth.myapp.localhost"},
		{"host déclaré", published("app1-api", "api.app-1"), "feat-auth", "myapp", "api.app-1.feat-auth.myapp.localhost"},
		{"segments assainis", published("App_Web", ""), "feat/auth", "My.App", "app-web.feat-auth.my-app.localhost"},
		{"job sans url", domain.JobConfig{Name: "db"}, "feat-auth", "myapp", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RouteHost(RouteHostParams{Job: tt.job, Worktree: tt.worktree, Project: tt.project})
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRouteHostOrderIsolatesCookies(t *testing.T) {
	job := domain.JobConfig{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}}

	a := RouteHost(RouteHostParams{Job: job, Worktree: "feat-auth", Project: "myapp"})
	b := RouteHost(RouteHostParams{Job: job, Worktree: "main", Project: "myapp"})

	// Le parent commun doit être le projet, jamais le job : sinon un cookie posé
	// sur le parent fuiterait d'un worktree à l'autre, ce que la feature répare.
	if a == b {
		t.Fatalf("two worktrees must not share a hostname, both are %q", a)
	}
	if parent(a) == parent(b) {
		t.Errorf("worktrees must not share their cookie parent: %q and %q", a, b)
	}
}

func parent(host string) string {
	for i := 0; i < len(host); i++ {
		if host[i] == '.' {
			return host[i+1:]
		}
	}
	return host
}

func TestProxyPort(t *testing.T) {
	on, off := true, false

	tests := []struct {
		name string
		cfg  domain.GlobalConfig
		want int
	}{
		{"absent : le défaut", domain.GlobalConfig{}, domain.ProxyDefaultPort},
		{"port choisi", domain.GlobalConfig{Proxy: domain.ProxyConfig{Port: 5000}}, 5000},
		{"éteint explicitement", domain.GlobalConfig{Proxy: domain.ProxyConfig{Enabled: &off, Port: 5000}}, 0},
		{"allumé sans port", domain.GlobalConfig{Proxy: domain.ProxyConfig{Enabled: &on}}, domain.ProxyDefaultPort},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProxyPort(tt.cfg); got != tt.want {
				t.Errorf("ProxyPort = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestValidateProxy(t *testing.T) {
	for port, ok := range map[int]bool{0: true, 1: true, 11080: true, 65535: true, -1: false, 65536: false, 70000: false} {
		err := ValidateProxy(domain.ProxyConfig{Port: port})
		if (err == nil) != ok {
			t.Errorf("ValidateProxy(port %d) = %v, want ok=%v", port, err, ok)
		}
	}
}

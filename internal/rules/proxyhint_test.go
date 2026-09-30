package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestProxyInstallHintLines(t *testing.T) {
	published := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", URL: &domain.JobURLConfig{Port: "PORT"}}}}
	silent := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "db"}}}

	tests := []struct {
		name      string
		config    domain.RunConfig
		status    domain.ProxyStatus
		wantLines bool
		wantCmd   bool
	}{
		{name: "aucun job ne publie : rien à dire", config: silent, status: domain.ProxyStatus{Supported: true}},
		{name: "déjà installé : rien à dire", config: published, status: domain.ProxyStatus{Supported: true, Installed: true}},
		{name: "publié et pas installé", config: published, status: domain.ProxyStatus{Supported: true}, wantLines: true, wantCmd: true},
		{name: "plateforme non supportée : on informe sans proposer", config: published, status: domain.ProxyStatus{}, wantLines: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := ProxyInstallHintLines(ProxyInstallHintParams{Config: tt.config, Status: tt.status, ExampleURL: "http://web.f.app.localhost:4000"})

			if got := len(lines) > 0; got != tt.wantLines {
				t.Fatalf("lines: got %v, want %v (%v)", got, tt.wantLines, lines)
			}
			if got := strings.Contains(strings.Join(lines, "\n"), "wtm run proxy install"); got != tt.wantCmd {
				t.Errorf("mention de la commande: got %v, want %v", got, tt.wantCmd)
			}
		})
	}
}

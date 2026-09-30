package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestNeedsDevOrigins(t *testing.T) {
	nextJob := domain.JobConfig{
		Name:  "web",
		Cmd:   "pnpm run dev --port ${PORT}",
		Ports: map[string]int{"PORT": 3000},
		URL:   &domain.JobURLConfig{Port: "PORT"},
	}

	tests := []struct {
		name   string
		job    domain.JobConfig
		source string
		want   bool
	}{
		{"next sans allowedDevOrigins", nextJob, "export default { reactStrictMode: true }\n", true},
		{"next avec allowedDevOrigins", nextJob, "export default { allowedDevOrigins: [\"*.localhost:4000\"] }\n", false},
		{"pas de next.config", nextJob, "", false},
		{"job qui ne publie rien", domain.JobConfig{Name: "db", Cmd: "docker compose up"}, "export default {}\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsDevOrigins(NeedsDevOriginsParams{Job: tt.job, ConfigSource: tt.source})
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

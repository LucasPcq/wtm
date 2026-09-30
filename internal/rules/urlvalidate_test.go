package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestValidateRunPortsURL(t *testing.T) {
	tests := []struct {
		name string
		cfg  domain.RunConfig
		want string
	}{
		{
			name: "url.port doit nommer un port déclaré",
			cfg: domain.RunConfig{Jobs: []domain.JobConfig{
				{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "HTTP_PORT"}},
			}},
			want: `job "web": url.port names HTTP_PORT, which the job does not declare`,
		},
		{
			name: "url.host doit être une suite de labels",
			cfg: domain.RunConfig{Jobs: []domain.JobConfig{
				{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT", Host: "Web_1"}},
			}},
			want: `job "web": url.host "Web_1" is not a valid hostname`,
		},
		{
			name: "deux jobs ne peuvent pas revendiquer le même hôte",
			cfg: domain.RunConfig{Jobs: []domain.JobConfig{
				{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT", Host: "app"}},
				{Name: "bo", Ports: map[string]int{"PORT": 3001}, URL: &domain.JobURLConfig{Port: "PORT", Host: "app"}},
			}},
			want: `jobs "web" and "bo" both publish host "app"`,
		},
		{
			name: "le nom du job sert d'hôte par défaut et collisionne pareil",
			cfg: domain.RunConfig{Jobs: []domain.JobConfig{
				{Name: "app_web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}},
				{Name: "app-web", Ports: map[string]int{"PORT": 3001}, URL: &domain.JobURLConfig{Port: "PORT"}},
			}},
			want: `both publish host "app-web"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateRunPorts(tt.cfg)
			if !containsSubstring(errs, tt.want) {
				t.Errorf("errors %v, want one containing %q", errs, tt.want)
			}
		})
	}
}

func TestValidateRunPortsURLAccepts(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "app1-web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}},
		{Name: "app1-api", Ports: map[string]int{"PORT": 4000}, URL: &domain.JobURLConfig{Port: "PORT", Host: "api.app-1"}},
		{Name: "db", Ports: map[string]int{"PG_PORT": 5432}},
	}}
	if errs := ValidateRunPorts(cfg); len(errs) > 0 {
		t.Errorf("a valid config must not be refused, got %v", errs)
	}
}

func containsSubstring(errs []string, want string) bool {
	for _, e := range errs {
		if strings.Contains(e, want) {
			return true
		}
	}
	return false
}

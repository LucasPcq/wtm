package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestJobURL(t *testing.T) {
	web := domain.JobConfig{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}}

	tests := []struct {
		name       string
		job        domain.JobConfig
		ports      map[string]int
		host       string
		publicPort int
		want       string
	}{
		{name: "port résolu du worktree", job: web, ports: map[string]int{"PORT": 3010}, want: "http://localhost:3010"},
		{name: "worktree principal", job: web, ports: map[string]int{"PORT": 3000}, want: "http://localhost:3000"},
		{name: "job sans url", job: domain.JobConfig{Name: "db", Ports: map[string]int{"PG_PORT": 5432}}, ports: map[string]int{"PG_PORT": 5442}, want: ""},
		{name: "port absent des résolus", job: web, ports: map[string]int{"OTHER": 9000}, want: ""},
		{name: "aucun port résolu", job: web, want: ""},
		{
			name:       "forme nommée quand le proxy sert",
			job:        web,
			ports:      map[string]int{"PORT": 3010},
			host:       "web.feat-auth.myapp.localhost",
			publicPort: 4000,
			want:       "http://web.feat-auth.myapp.localhost:4000",
		},
		{
			name:       "port public 80 : l'URL ne porte aucun port",
			job:        web,
			ports:      map[string]int{"PORT": 3010},
			host:       "web.feat-auth.myapp.localhost",
			publicPort: 80,
			want:       "http://web.feat-auth.myapp.localhost",
		},
		{
			name:       "port public haut : le port reste dans l'URL",
			job:        web,
			ports:      map[string]int{"PORT": 3010},
			host:       "web.feat-auth.myapp.localhost",
			publicPort: 4001,
			want:       "http://web.feat-auth.myapp.localhost:4001",
		},
		{
			name:  "proxy éteint : la forme directe est la réponse honnête",
			job:   web,
			ports: map[string]int{"PORT": 3010},
			host:  "web.feat-auth.myapp.localhost",
			want:  "http://localhost:3010",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JobURL(JobURLParams{Job: tt.job, Ports: tt.ports, Host: tt.host, PublicPort: tt.publicPort})
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseJobURLRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    *domain.JobURLConfig
		wantErr bool
	}{
		{name: "vide ne publie rien", in: "  "},
		{name: "port seul", in: "PORT", want: &domain.JobURLConfig{Port: "PORT"}},
		{name: "port et hôte", in: "PORT api.app-1", want: &domain.JobURLConfig{Port: "PORT", Host: "api.app-1"}},
		{name: "port invalide", in: "not-a-var", wantErr: true},
		{name: "hôte invalide", in: "PORT Web_1", wantErr: true},
		{name: "trop de valeurs", in: "PORT a b", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseJobURL(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseJobURL(%q) = %+v, want an error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseJobURL(%q): %v", tt.in, err)
			}
			if (got == nil) != (tt.want == nil) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
			if got == nil {
				return
			}
			if *got != *tt.want {
				t.Fatalf("got %+v, want %+v", *got, *tt.want)
			}
			// A wizard pre-fills from what the job already declares, so the line
			// it shows has to parse back to the same thing.
			if again, _ := ParseJobURL(FormatJobURL(got)); *again != *got {
				t.Errorf("round trip gave %+v, want %+v", *again, *got)
			}
		})
	}
}

func TestJobOrigin(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		publicPort int
		directPort int
		want       string
	}{
		{name: "nom sur le 80", host: "web.f.app.localhost", publicPort: 80, directPort: 3010, want: "http://web.f.app.localhost"},
		{name: "nom sur un port haut", host: "web.f.app.localhost", publicPort: 4000, directPort: 3010, want: "http://web.f.app.localhost:4000"},
		{name: "pas de proxy : la forme directe", host: "web.f.app.localhost", directPort: 3010, want: "http://localhost:3010"},
		{name: "pas d'hôte : la forme directe", publicPort: 80, directPort: 3010, want: "http://localhost:3010"},
		{name: "rien à publier", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JobOrigin(JobOriginParams{Host: tt.host, PublicPort: tt.publicPort, DirectPort: tt.directPort})
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

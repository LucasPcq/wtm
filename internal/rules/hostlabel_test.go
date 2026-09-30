package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestHostLabel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"déjà un label", "web", "web"},
		{"majuscules abaissées", "Web-API", "web-api"},
		{"scope npm et slash", "@acme/web", "acme-web"},
		{"underscore interdit en hôte", "app_web", "app-web"},
		{"séquences réduites", "app///web", "app-web"},
		{"tirets de bord coupés", "-web-", "web"},
		{"tronqué à 63", "a" + strings.Repeat("b", 79), "a" + strings.Repeat("b", 62)},
		{"vide se replie", "///", domain.HostLabelFallback},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HostLabel(tt.in); got != tt.want {
				t.Errorf("HostLabel(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsHostLabels(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"un label", "web", true},
		{"deux labels", "web.app-1", true},
		{"vide", "", false},
		{"majuscule refusée", "Web", false},
		{"underscore refusé", "app_web", false},
		{"label vide", "web..app", false},
		{"tiret de tête", "-web", false},
		{"tiret de queue", "web-", false},
		{"label trop long", strings.Repeat("a", 64), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHostLabels(tt.in); got != tt.want {
				t.Errorf("IsHostLabels(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

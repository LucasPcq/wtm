package components

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/rules"
)

func TestURLAtFindsTheAddressUnderTheClick(t *testing.T) {
	view := "JOBS\n\x1b[2m│ web  \x1b[0m" + rules.LinkURLs("http://localhost:4012") + " · app_x"
	cases := map[string]struct {
		x, y  int
		want  string
		found bool
	}{
		"on its first column": {x: 7, y: 1, want: "http://localhost:4012", found: true},
		"on its last column":  {x: 27, y: 1, want: "http://localhost:4012", found: true},
		"just past it":        {x: 28, y: 1},
		"before it":           {x: 3, y: 1},
		"another line":        {x: 7, y: 0},
		"below the frame":     {x: 7, y: 5},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, found := URLAt(URLAtParams{View: view, X: c.x, Y: c.y})
			if got != c.want || found != c.found {
				t.Errorf("URLAt = %q, %v — want %q, %v", got, found, c.want, c.found)
			}
		})
	}
}

// A display cut the address short: following the start of it lands elsewhere.
func TestURLAtIgnoresATruncatedAddress(t *testing.T) {
	if got, found := URLAt(URLAtParams{View: "http://crm-api-dev.env-te…", X: 3}); found {
		t.Errorf("URLAt = %q, want nothing on a truncated address", got)
	}
}

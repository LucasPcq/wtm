package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

var surfaceNames = map[domain.RunSurface]string{
	domain.RunSurfaceView:    "view",
	domain.RunSurfaceStream:  "stream",
	domain.RunSurfaceMachine: "machine",
}

func TestDecideRunSurface(t *testing.T) {
	cases := []struct {
		name   string
		params RunSurfaceParams
		want   domain.RunSurface
	}{
		{
			name:   "attached on a terminal opens the view",
			params: RunSurfaceParams{TTY: true, Format: domain.OutputText},
			want:   domain.RunSurfaceView,
		},
		{
			name:   "detached keeps the terminal it was given",
			params: RunSurfaceParams{Detach: true, TTY: true, Format: domain.OutputText},
			want:   domain.RunSurfaceStream,
		},
		{
			name:   "a task is read back from the scrollback",
			params: RunSurfaceParams{Inline: true, TTY: true, Format: domain.OutputText},
			want:   domain.RunSurfaceStream,
		},
		{
			name:   "a pipe has no screen to take over",
			params: RunSurfaceParams{TTY: false, Format: domain.OutputText},
			want:   domain.RunSurfaceStream,
		},
		{
			// The one that matters most: a terminal is not a reason to draw on it
			// when the caller asked for a document.
			name:   "json on a terminal is still a document",
			params: RunSurfaceParams{TTY: true, Format: domain.OutputJSON},
			want:   domain.RunSurfaceMachine,
		},
		{
			name:   "json wins over the removal flag",
			params: RunSurfaceParams{Detach: true, TTY: true, Format: domain.OutputJSON},
			want:   domain.RunSurfaceMachine,
		},
		{
			name:   "json wins over a task",
			params: RunSurfaceParams{Inline: true, TTY: true, Format: domain.OutputJSON},
			want:   domain.RunSurfaceMachine,
		},
		{
			name:   "json in a pipe stays a document",
			params: RunSurfaceParams{Format: domain.OutputJSON},
			want:   domain.RunSurfaceMachine,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecideRunSurface(tc.params)
			if got != tc.want {
				t.Fatalf("surface = %s, want %s", surfaceNames[got], surfaceNames[tc.want])
			}
		})
	}
}

// The view is the only surface that takes the terminal over, so every input
// that removes one of its three conditions has to move the answer away from it.
func TestDecideRunSurfaceNeedsAllThreeConditions(t *testing.T) {
	attached := RunSurfaceParams{TTY: true, Format: domain.OutputText}
	if DecideRunSurface(attached) != domain.RunSurfaceView {
		t.Fatal("the attached baseline no longer opens the view")
	}

	for _, tc := range []struct {
		what   string
		params RunSurfaceParams
	}{
		{"no terminal", RunSurfaceParams{Format: domain.OutputText}},
		{"json", RunSurfaceParams{TTY: true, Format: domain.OutputJSON}},
		{"detached", RunSurfaceParams{TTY: true, Format: domain.OutputText, Detach: true}},
		{"inline", RunSurfaceParams{TTY: true, Format: domain.OutputText, Inline: true}},
	} {
		if got := DecideRunSurface(tc.params); got == domain.RunSurfaceView {
			t.Errorf("%s still opened the view", tc.what)
		}
	}
}

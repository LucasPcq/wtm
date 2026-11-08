package output

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestASpanIsItsLargestWholeUnit(t *testing.T) {
	cases := map[string]struct {
		d    time.Duration
		want string
	}{
		"seconds":        {d: 42 * time.Second, want: "42s"},
		"minutes":        {d: 12*time.Minute + 59*time.Second, want: "12m"},
		"hours":          {d: 3 * time.Hour, want: "3h"},
		"days":           {d: 50 * time.Hour, want: "2d"},
		"never_negative": {d: -time.Second, want: "0s"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, Span(c.d))
		})
	}
}

func TestCountedSaysOneOrMany(t *testing.T) {
	assert.Equal(t, "1 worktree", Counted(1, "worktree", "worktrees"))
	assert.Equal(t, "3 worktrees", Counted(3, "worktree", "worktrees"))
}

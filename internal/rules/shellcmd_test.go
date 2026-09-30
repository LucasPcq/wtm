package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestIsBlankCommand(t *testing.T) {
	cases := []struct {
		line  string
		blank bool
	}{
		{"", true},
		{"   ", true},
		{"\t\n", true},
		{"echo hi", false},
		{"  echo hi  ", false},
	}

	for _, c := range cases {
		if got := IsBlankCommand(c.line); got != c.blank {
			t.Errorf("IsBlankCommand(%q) = %v, want %v", c.line, got, c.blank)
		}
	}
}

// TestShellCommandKeepsLineIntact is the whole point of the rule: the line is
// handed to the shell as one argument, never split, so quoting and operators
// survive to be interpreted.
func TestShellCommandKeepsLineIntact(t *testing.T) {
	line := `pnpm install && node -e 'console.log(1)' --port ${PORT}`

	spec := ShellCommand(line)

	if spec.Name != domain.ShellBin {
		t.Errorf("Name = %q, want %q", spec.Name, domain.ShellBin)
	}
	want := []string{domain.ShellCommandFlag, line}
	if len(spec.Args) != len(want) {
		t.Fatalf("Args = %v, want %v", spec.Args, want)
	}
	for i := range want {
		if spec.Args[i] != want[i] {
			t.Errorf("Args[%d] = %q, want %q", i, spec.Args[i], want[i])
		}
	}
}

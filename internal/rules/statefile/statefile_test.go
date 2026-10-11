package statefile_test

import (
	"errors"
	"testing"

	domainstatefile "github.com/LucasPcq/wtm/internal/domain/statefile"
	"github.com/LucasPcq/wtm/internal/rules/statefile"
)

func TestNewerReadsAMissingSchemaAsOne(t *testing.T) {
	cases := []struct {
		found, supported int
		want             bool
	}{
		{found: 0, supported: 1, want: false},
		{found: 1, supported: 1, want: false},
		{found: 2, supported: 1, want: true},
		{found: 1, supported: 2, want: false},
		{found: 0, supported: 2, want: false},
	}
	for _, tc := range cases {
		if got := statefile.Newer(statefile.SchemaParams{Found: tc.found, Supported: tc.supported}); got != tc.want {
			t.Errorf("Newer(found %d, supported %d) = %v, want %v", tc.found, tc.supported, got, tc.want)
		}
	}
}

func TestWrittenOmitsSchemaOne(t *testing.T) {
	if got := statefile.Written(1); got != 0 {
		t.Errorf("Written(1) = %d, want 0", got)
	}
	if got := statefile.Written(3); got != 3 {
		t.Errorf("Written(3) = %d, want 3", got)
	}
}

func TestRefusalWrapsTheSentinelOnlyForANewerFile(t *testing.T) {
	newer := statefile.RefusalParams{Path: "run.toml", Schema: statefile.SchemaParams{Found: 2, Supported: 1}}
	if err := statefile.Refusal(newer); !errors.Is(err, domainstatefile.ErrSchemaNewer) {
		t.Errorf("Refusal = %v, want %v", err, domainstatefile.ErrSchemaNewer)
	}
	current := statefile.RefusalParams{Path: "run.toml", Schema: statefile.SchemaParams{Found: 1, Supported: 1}}
	if err := statefile.Refusal(current); err != nil {
		t.Errorf("Refusal = %v, want nil", err)
	}
}

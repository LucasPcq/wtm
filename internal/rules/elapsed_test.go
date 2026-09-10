package rules_test

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/rules"
)

func TestParseElapsedReadsEveryFormPsPrints(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"00:07", 7 * time.Second},
		{"12:34", 12*time.Minute + 34*time.Second},
		{"01:02:03", time.Hour + 2*time.Minute + 3*time.Second},
		{"12-20:43:08", 12*24*time.Hour + 20*time.Hour + 43*time.Minute + 8*time.Second},
		{"  03:00  ", 3 * time.Minute},
	}

	for _, tc := range cases {
		got, ok := rules.ParseElapsed(tc.in)
		if !ok {
			t.Errorf("ParseElapsed(%q) refused a form ps prints", tc.in)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseElapsed(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseElapsedRefusesWhatItCannotRead(t *testing.T) {
	for _, in := range []string{"", "-", "7", "aa:bb", "1:2:3:4", "12-", "-1:00"} {
		if _, ok := rules.ParseElapsed(in); ok {
			t.Errorf("ParseElapsed(%q) accepted an unreadable field: a guess here kills a stranger", in)
		}
	}
}

package output

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/env"

	"github.com/stretchr/testify/assert"
)

func TestTheSettlementNoteCountsPortsAndOwnedValues(t *testing.T) {
	rewrite := env.Entry{Status: env.StatusRewrite}
	unchanged := env.Entry{Status: env.StatusUnchanged}
	written := env.OwnedEntry{Key: "REALM", Changed: true}
	kept := env.OwnedEntry{Key: "COMPOSE_PROJECT_NAME"}
	cases := []struct {
		name string
		plan env.Redacted
		want string
	}{
		{name: "nothing_moved", plan: env.Redacted{Offset: 20, Entries: []env.Entry{unchanged}, Owned: []env.OwnedEntry{kept}}, want: ""},
		{name: "one_port", plan: env.Redacted{Offset: 20, Entries: []env.Entry{rewrite, unchanged}}, want: "1 port settled (offset +20)"},
		{name: "several_ports", plan: env.Redacted{Offset: 10, Entries: []env.Entry{rewrite, rewrite}}, want: "2 ports settled (offset +10)"},
		{name: "owned_values_only", plan: env.Redacted{Owned: []env.OwnedEntry{written, kept, written}}, want: "2 owned values written"},
		{name: "both", plan: env.Redacted{Offset: 10, Entries: []env.Entry{rewrite}, Owned: []env.OwnedEntry{written}}, want: "1 port settled (offset +10) · 1 owned value written"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SettlementNote(tc.plan))
		})
	}
}

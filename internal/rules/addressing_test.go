package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestEffectiveAddressing(t *testing.T) {
	cases := []struct {
		name string
		cfg  domain.RunConfig
		want domain.Addressing
	}{
		{"unset defaults to names", domain.RunConfig{}, domain.AddressingNames},
		{"names", domain.RunConfig{Addressing: domain.AddressingNames}, domain.AddressingNames},
		{"ports opts out", domain.RunConfig{Addressing: domain.AddressingPorts}, domain.AddressingPorts},
		{"unknown falls back to the default", domain.RunConfig{Addressing: "nope"}, domain.AddressingNames},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.EffectiveAddressing(tc.cfg); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateAddressing(t *testing.T) {
	for _, value := range []domain.Addressing{"", domain.AddressingPorts, domain.AddressingNames} {
		if errs := rules.ValidateAddressing(domain.RunConfig{Addressing: value}); len(errs) > 0 {
			t.Fatalf("addressing %q rejected: %v", value, errs)
		}
	}
	errs := rules.ValidateAddressing(domain.RunConfig{Addressing: "name"})
	if len(errs) != 1 {
		t.Fatalf("a typo must be refused, got %v", errs)
	}
}

func TestBulkSettlesMainOnlyBackToPorts(t *testing.T) {
	if !rules.BulkSettlesMain(domain.AddressingPorts) {
		t.Error("a bulk pass must be able to bring main back to ports")
	}
	if rules.BulkSettlesMain(domain.AddressingNames) {
		t.Error("a bulk pass must never move main onto names")
	}
}

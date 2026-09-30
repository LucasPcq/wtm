package rules

import (
	"strings"
	"testing"
)

func TestLinkURLsWrapsEveryAddressAndNothingElse(t *testing.T) {
	got := LinkURLs("web  http://localhost:4012 · app_x\napi  http://api.x.localhost:11080")
	for _, url := range []string{"http://localhost:4012", "http://api.x.localhost:11080"} {
		if !strings.Contains(got, "\x1b]8;;"+url+"\x1b\\"+url+"\x1b]8;;\x1b\\") {
			t.Errorf("%s not linked in %q", url, got)
		}
	}
	if !strings.Contains(got, " · app_x") {
		t.Errorf("the text around the address changed: %q", got)
	}
}

func TestLinkURLsLeavesPlainTextAndExistingLinksAlone(t *testing.T) {
	for _, text := range []string{"no address here", "3 urls", LinkURLs("http://localhost:1")} {
		if got := LinkURLs(text); got != text {
			t.Errorf("LinkURLs(%q) = %q, want it unchanged", text, got)
		}
	}
}

func TestLinkURLsSkipsATruncatedAddress(t *testing.T) {
	text := "http://crm-api-dev.env-te…"
	if got := LinkURLs(text); got != text {
		t.Errorf("LinkURLs = %q, want a truncated address left unlinked", got)
	}
}

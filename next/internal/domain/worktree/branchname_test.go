package worktree

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestANameGitAcceptsHasNoProblem(t *testing.T) {
	for _, name := range []string{"main", "feat/login", "fix/a-b_c.d", "v1.2", "user@host", "a/b/c"} {
		assert.NoError(t, BranchProblem(name), name)
	}
}

func TestANameGitRefusesSaysWhy(t *testing.T) {
	cases := []struct {
		name   string
		branch string
		want   string
	}{
		{"blank", "  ", `"  " is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"at_sign", "@", `"@" is not a valid branch name: it is reserved by git`},
		{"head", "HEAD", `"HEAD" is not a valid branch name: it is reserved by git`},
		{"leading_dash", "-x", `"-x" is not a valid branch name: it cannot start with -`},
		{"leading_slash", "/a", `"/a" is not a valid branch name: a / cannot start or end it, or follow another`},
		{"trailing_slash", "a/", `"a/" is not a valid branch name: a / cannot start or end it, or follow another`},
		{"double_slash", "a//b", `"a//b" is not a valid branch name: a / cannot start or end it, or follow another`},
		{"trailing_dot", "a.", `"a." is not a valid branch name: it cannot hold .. or end with .`},
		{"double_dot", "bad..name", `"bad..name" is not a valid branch name: it cannot hold .. or end with .`},
		{"at_brace", "a@{1}", `"a@{1}" is not a valid branch name: it cannot hold @{`},
		{"space", "a b", `"a b" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"tab", "tab\tname", `"tab\tname" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"tilde", "a~1", `"a~1" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"caret", "a^", `"a^" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"colon", "a:b", `"a:b" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"question_mark", "a?", `"a?" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"asterisk", "a*", `"a*" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"open_bracket", "a[b", `"a[b" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"backslash", `a\b`, `"a\\b" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"hidden_part", ".hidden", `".hidden" is not a valid branch name: no part between slashes may start with . or end with .lock`},
		{"nested_hidden_part", "a/.b", `"a/.b" is not a valid branch name: no part between slashes may start with . or end with .lock`},
		{"lock_suffix", "x.lock", `"x.lock" is not a valid branch name: no part between slashes may start with . or end with .lock`},
		{"nested_lock_suffix", "a/x.lock/b", `"a/x.lock/b" is not a valid branch name: no part between slashes may start with . or end with .lock`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.EqualError(t, BranchProblem(tc.branch), tc.want)
		})
	}
}

func TestAnEmptyNameIsLeftToTheCallerAsTheEngineDid(t *testing.T) {
	assert.NoError(t, BranchProblem(""))
}

func TestABlankNameIsBlank(t *testing.T) {
	assert.True(t, BlankBranch("  \t"))
	assert.False(t, BlankBranch(" feat/x "))
}

func TestAPathSegmentTurnsEachSlashIntoADash(t *testing.T) {
	cases := map[string]string{"feature/login": "feature-login", "fix/auth/step-2": "fix-auth-step-2", "simple": "simple", "a/b/c/d": "a-b-c-d"}
	for name, want := range cases {
		assert.Equal(t, want, PathSegment(name), name)
	}
}

func TestASlugIsWhatDockerAcceptsAsAProjectName(t *testing.T) {
	cases := map[string]string{
		"feat/x":     "feat-x",
		"Feat/Login": "feat-login",
		"feat/a_b":   "feat-a_b",
		"_-feat":     "feat",
		"feat.x":     "feat-x",
		"///":        "wtm",
	}
	for name, want := range cases {
		assert.Equal(t, want, Slug(name), name)
	}
}

func TestAHostLabelIsOneDNSLabel(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"already_label", "web", "web"},
		{"uppercase_lowered", "Web-API", "web-api"},
		{"npm_scope_and_slash", "@acme/web", "acme-web"},
		{"underscore_replaced", "app_web", "app-web"},
		{"runs_collapsed", "app///web", "app-web"},
		{"edge_dashes_trimmed", "-web-", "web"},
		{"cut_at_63", "a" + strings.Repeat("b", 79), "a" + strings.Repeat("b", 62)},
		{"empty_falls_back", "///", "wtm"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, HostLabel(tc.in))
		})
	}
}

func TestAStateSegmentEscapesTheSlashesAPathSegmentTurnsIntoDashes(t *testing.T) {
	assert.Equal(t, "feat%2Fx", StateSegment("feat/x"))
	assert.Equal(t, "feat-x", StateSegment("feat-x"))
	assert.NotEqual(t, StateSegment("feat/x"), StateSegment("feat-x"))
}

func TestABranchClashesWithALiveOneSharingADerivedName(t *testing.T) {
	cases := []struct {
		name   string
		branch string
		live   []string
		want   Clash
		found  bool
	}{
		{name: "dot_and_slash_share_slug", branch: "feat.x", live: []string{"main", "feat/x"}, want: Clash{Branch: "feat/x", Name: "feat-x"}, found: true},
		{name: "case_folded", branch: "Feat/X", live: []string{"feat/x"}, want: Clash{Branch: "feat/x", Name: "feat-x"}, found: true},
		{name: "underscore_in_slug_not_label", branch: "feat/a_b", live: []string{"feat/a-b"}, want: Clash{Branch: "feat/a-b", Name: "feat-a-b"}, found: true},
		{name: "self_no_clash", branch: "feat/x", live: []string{"feat/x"}},
		{name: "distinct_names", branch: "feat/y", live: []string{"main", "feat/x"}},
		{name: "detached_has_no_name", branch: "feat/x", live: []string{""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clash, found := ClashWith(tc.branch, tc.live)

			assert.Equal(t, tc.found, found)
			assert.Equal(t, tc.want, clash)
		})
	}
}

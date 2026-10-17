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
		name string
		want string
	}{
		{"  ", `"  " is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"@", `"@" is not a valid branch name: it is reserved by git`},
		{"HEAD", `"HEAD" is not a valid branch name: it is reserved by git`},
		{"-x", `"-x" is not a valid branch name: it cannot start with -`},
		{"/a", `"/a" is not a valid branch name: a / cannot start or end it, or follow another`},
		{"a/", `"a/" is not a valid branch name: a / cannot start or end it, or follow another`},
		{"a//b", `"a//b" is not a valid branch name: a / cannot start or end it, or follow another`},
		{"a.", `"a." is not a valid branch name: it cannot hold .. or end with .`},
		{"bad..name", `"bad..name" is not a valid branch name: it cannot hold .. or end with .`},
		{"a@{1}", `"a@{1}" is not a valid branch name: it cannot hold @{`},
		{"a b", `"a b" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"tab\tname", `"tab\tname" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"a~1", `"a~1" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"a^", `"a^" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"a:b", `"a:b" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"a?", `"a?" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"a*", `"a*" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{"a[b", `"a[b" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{`a\b`, `"a\\b" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`},
		{".hidden", `".hidden" is not a valid branch name: no part between slashes may start with . or end with .lock`},
		{"a/.b", `"a/.b" is not a valid branch name: no part between slashes may start with . or end with .lock`},
		{"x.lock", `"x.lock" is not a valid branch name: no part between slashes may start with . or end with .lock`},
		{"a/x.lock/b", `"a/x.lock/b" is not a valid branch name: no part between slashes may start with . or end with .lock`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.EqualError(t, BranchProblem(tc.name), tc.want)
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
		{"already a label", "web", "web"},
		{"uppercase lowered", "Web-API", "web-api"},
		{"npm scope and slash", "@acme/web", "acme-web"},
		{"no underscore in a host", "app_web", "app-web"},
		{"runs collapsed", "app///web", "app-web"},
		{"edge dashes trimmed", "-web-", "web"},
		{"cut at 63", "a" + strings.Repeat("b", 79), "a" + strings.Repeat("b", 62)},
		{"empty falls back", "///", "wtm"},
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
		{name: "a dot and a slash share the slug", branch: "feat.x", live: []string{"main", "feat/x"}, want: Clash{Branch: "feat/x", Name: "feat-x"}, found: true},
		{name: "case is folded", branch: "Feat/X", live: []string{"feat/x"}, want: Clash{Branch: "feat/x", Name: "feat-x"}, found: true},
		{name: "an underscore survives the slug, not the host label", branch: "feat/a_b", live: []string{"feat/a-b"}, want: Clash{Branch: "feat/a-b", Name: "feat-a-b"}, found: true},
		{name: "the branch itself is no clash", branch: "feat/x", live: []string{"feat/x"}},
		{name: "distinct names", branch: "feat/y", live: []string{"main", "feat/x"}},
		{name: "a detached worktree has no name", branch: "feat/x", live: []string{""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clash, found := ClashWith(tc.branch, tc.live)

			assert.Equal(t, tc.found, found)
			assert.Equal(t, tc.want, clash)
		})
	}
}

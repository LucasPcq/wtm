package branch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestANameGitAcceptsHasNoProblem(t *testing.T) {
	for _, name := range []string{"main", "feat/login", "fix/a-b_c.d", "v1.2", "user@host", "a/b/c"} {
		assert.NoError(t, Problem(name), name)
	}
}

func TestANameGitRefusesSaysWhy(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"", `"" is not a valid branch name: it is empty`},
		{"  ", `"  " is not a valid branch name: it is empty`},
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
			assert.EqualError(t, Problem(tc.name), tc.want)
		})
	}
}

func TestABlankNameIsBlank(t *testing.T) {
	assert.True(t, Blank("  \t"))
	assert.False(t, Blank(" feat/x "))
}

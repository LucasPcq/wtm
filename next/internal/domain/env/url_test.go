package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTheCredentialsOfAValueAreMasked(t *testing.T) {
	cases := map[string]struct{ value, want string }{
		"userinfo":       {value: "postgres://app:hunter2@localhost:5432/app", want: "postgres://•••@localhost:5432/app"},
		"query_password": {value: "http://localhost:8025/?user=a&password=hunter2&x=1", want: "http://localhost:8025/?user=a&password=•••&x=1"},
		"list":           {value: "http://u:p@a:1, http://b:2?Token=t", want: "http://•••@a:1, http://b:2?Token=•••"},
		"none":           {value: "http://localhost:3000", want: "http://localhost:3000"},
		"reserved_chars": {value: "postgres://app:p/ss?x#y@localhost:5432/app", want: "postgres://•••@localhost:5432/app"},
		"non_ascii":      {value: "http://İKelvinK:1/?PASSWORD=ü&q=İ", want: "http://İKelvinK:1/?PASSWORD=•••&q=İ"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, WithoutCredentials(c.value))
		})
	}
}

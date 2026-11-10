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
		"access_token":   {value: "http://localhost:1/?access_token=abc", want: "http://localhost:1/?access_token=•••"},
		"api_key":        {value: "http://localhost:1/?x=1&api_key=abc&y=2", want: "http://localhost:1/?x=1&api_key=•••&y=2"},
		"client_secret":  {value: "http://localhost:1/?client_secret=abc", want: "http://localhost:1/?client_secret=•••"},
		"fragment":       {value: "http://localhost:1/#token=abc", want: "http://localhost:1/#token=•••"},
		"ado_net":        {value: "Server=localhost,1433;User Id=sa;Password=Secr3t", want: "Server=localhost,1433;User Id=sa;Password=•••"},
		"libpq":          {value: "host=localhost port=5432 password=Secr3t dbname=app", want: "host=localhost port=5432 password=••• dbname=app"},
		"at_the_start":   {value: "TOKEN=abc;host=x", want: "TOKEN=•••;host=x"},
		"in_a_path":      {value: "http://localhost:1/a/password=x", want: "http://localhost:1/a/password=x"},
		"single_quoted":  {value: "host=localhost password='p w' dbname=app", want: "host=localhost password='•••' dbname=app"},
		"double_quoted":  {value: `Password="a;b";Server=localhost`, want: `Password="•••";Server=localhost`},
		"braced":         {value: "Pwd={a;b};Server=localhost", want: "Pwd={•••};Server=localhost"},
		"unclosed":       {value: "password='a b", want: "password='•••"},
		"comma":          {value: "password=a,b;Server=x", want: "password=•••;Server=x"},
		"ez_connect":     {value: "scott/tiger@localhost:1521/XE", want: "•••@localhost:1521/XE"},
		"ez_connect_dsn": {value: "DSN=scott/tiger@db.local:1521/XE", want: "DSN=•••@db.local:1521/XE"},
		"no_login":       {value: "a/b@example", want: "a/b@example"},
		"key_alone":      {value: "http://localhost:1/?key=abc", want: "http://localhost:1/?key=•••"},
		"api_key_kinds":  {value: "APIKEY=a;access_key=b;Secret_Key=c;private_key=d", want: "APIKEY=•••;access_key=•••;Secret_Key=•••;private_key=•••"},
		"redirect_key":   {value: "http://localhost:1/?redirect_key=http://localhost:3001/x", want: "http://localhost:1/?redirect_key=http://localhost:3001/x"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, WithoutCredentials(c.value))
		})
	}
}

package env

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/stretchr/testify/assert"
)

func TestAnUnlinkedValueLooksLinkedWhenItHoldsAPortOrAnAddressOfThisMachine(t *testing.T) {
	run := project.Run{Jobs: []project.Job{{Name: "stack", Ports: map[string]int{"MAILPIT_UI": 8025}}}}
	cases := map[string]struct {
		value string
		like  Lookalike
		looks bool
	}{
		"address_on_a_port": {value: "http://localhost:8025", like: Lookalike{Job: "stack", Port: "MAILPIT_UI"}, looks: true},
		"port_alone":        {value: "8025", like: Lookalike{Job: "stack", Port: "MAILPIT_UI"}, looks: true},
		"address_elsewhere": {value: "http://127.0.0.1:9999/x", looks: true},
		"remote_address":    {value: "https://api.stripe.com", looks: false},
		"number_in_text":    {value: "timeout 8025ms", looks: false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			like, looks := LooksLinked(c.value, run)

			assert.Equal(t, c.looks, looks)
			assert.Equal(t, c.like, like)
		})
	}
}

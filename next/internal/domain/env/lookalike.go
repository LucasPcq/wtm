package env

import (
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// A Lookalike is a port run.toml declares that a value seems to hold: the job and the name it declares it under, empty when the value holds an address of this machine on no declared port.
type Lookalike struct {
	Job  string `json:"job,omitempty"`
	Port string `json:"port,omitempty"`
}

// LooksLinked reports whether value, of a key no link of run.toml follows, looks like one it should: a port a job declares, alone or in an address of this machine, or an address of this machine on another port; it names the job and the port it looks like.
func LooksLinked(value string, run project.Run) (Lookalike, bool) {
	ports := []int{}
	if n, err := strconv.Atoi(value); err == nil {
		ports = append(ports, n)
	}
	for _, element := range splitList(value) {
		if _, authority, ok := splitOrigin(element.value); ok {
			if host, port := splitHostPort(authority); isLoopbackHost(host) {
				ports = append(ports, port)
			}
		}
	}

	for _, port := range ports {
		for _, job := range run.Jobs {
			for name, base := range job.Ports {
				if port == base {
					return Lookalike{Job: job.Name, Port: name}, true
				}
			}
		}
	}
	return Lookalike{}, hasLoopbackOrigin(value)
}

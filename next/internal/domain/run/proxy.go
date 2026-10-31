package run

import (
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain/env"
)

// PrivilegedPort is the port a named URL announces once the redirection to
// the proxy is installed, and so spells no port.
const PrivilegedPort = 80

// A Route is a name the proxy serves a job's port under: the Job it names (a
// runner's children have routes of their own), the Host a browser asks for,
// and the variable of the job's environment that spells the Port.
type Route struct {
	Job  string
	Host string
	Port string
}

// An Address is a route the proxy serves, resolved: the Host a browser asks
// for, the Port on localhost that answers it, and the Job, Worktree and
// Project it names, for the page that lists what the proxy serves.
type Address struct {
	Host     string
	Port     int
	Job      string
	Worktree string
	Project  string
}

// Addresses returns the routes of the job its proxy serves, each one whose
// port its environment spells; a route whose port it does not is left out
// rather than served on a guess.
func (s Spec) Addresses() []Address {
	var addresses []Address
	for _, route := range s.Routes {
		port, err := strconv.Atoi(s.Env[route.Port])
		if err != nil || route.Host == "" {
			continue
		}
		addresses = append(addresses, Address{Host: route.Host, Port: port, Job: route.Job, Worktree: s.Env[env.WorktreeKey], Project: s.Env[env.ProjectKey]})
	}

	return addresses
}

// A Proxy is the proxy the daemon serves named URLs on: the Port it bound and
// the PublicPort a named URL announces, both 0 when it is off.
type Proxy struct {
	Port       int
	PublicPort int
}

// PublicPort returns the port a named URL announces for a proxy bound on
// bound, given the port of the proxy that answers on PrivilegedPort (probed,
// 0 for none): PrivilegedPort when the redirection reaches this proxy, bound
// otherwise, 0 when the proxy is off.
func PublicPort(bound, probed int) int {
	switch {
	case bound == 0:
		return 0
	case probed == bound:
		return PrivilegedPort
	}

	return bound
}

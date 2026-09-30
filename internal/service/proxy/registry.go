// Package proxy serves the named URLs of running jobs on one loopback port.
package proxy

import (
	"sort"
	"strings"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
)

// Registry is the routing table. It is a projection of the jobs the daemon is
// running, so it is only ever written from where those start and stop.
type Registry struct {
	mu     sync.RWMutex
	routes map[string]domain.ProxyRoute
}

func NewRegistry() *Registry {
	return &Registry{routes: map[string]domain.ProxyRoute{}}
}

func (r *Registry) Add(route domain.ProxyRoute) {
	if route.Host == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[strings.ToLower(route.Host)] = route
}

func (r *Registry) Remove(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.routes, strings.ToLower(host))
}

func (r *Registry) Lookup(host string) (domain.ProxyRoute, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	route, found := r.routes[strings.ToLower(host)]
	return route, found
}

func (r *Registry) List() []domain.ProxyRoute {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]domain.ProxyRoute, 0, len(r.routes))
	for _, route := range r.routes {
		out = append(out, route)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })
	return out
}

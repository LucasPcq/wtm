package proxy

import (
	"fmt"
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func route(host, target string) domain.ProxyRoute {
	return domain.ProxyRoute{Host: host, Target: target, Job: "web", Worktree: "feat", Project: "myapp"}
}

func TestRegistryLookupAndRemove(t *testing.T) {
	registry := NewRegistry()
	registry.Add(route("web.feat.myapp.localhost", "127.0.0.1:3010"))

	found, ok := registry.Lookup("web.feat.myapp.localhost")
	if !ok || found.Target != "127.0.0.1:3010" {
		t.Fatalf("Lookup = %+v, %v — want the route just added", found, ok)
	}
	if _, ok := registry.Lookup("nope.localhost"); ok {
		t.Error("an unknown host must not resolve")
	}

	registry.Remove("web.feat.myapp.localhost")
	if _, ok := registry.Lookup("web.feat.myapp.localhost"); ok {
		t.Error("a removed route must stop resolving")
	}
}

// A browser may send the Host in any case; a route registered once has to answer
// all of them.
func TestRegistryIsCaseInsensitive(t *testing.T) {
	registry := NewRegistry()
	registry.Add(route("Web.Feat.MyApp.localhost", "127.0.0.1:3010"))

	if _, ok := registry.Lookup("web.feat.myapp.localhost"); !ok {
		t.Error("lookup must not depend on the case the host was written in")
	}
}

func TestRegistryListIsSortedByHost(t *testing.T) {
	registry := NewRegistry()
	registry.Add(route("web.feat.myapp.localhost", "127.0.0.1:3010"))
	registry.Add(route("api.feat.myapp.localhost", "127.0.0.1:4010"))

	list := registry.List()
	if len(list) != 2 || list[0].Host != "api.feat.myapp.localhost" || list[1].Host != "web.feat.myapp.localhost" {
		t.Errorf("List = %+v, want it sorted by host", list)
	}
}

func TestRegistryIgnoresAHostlessRoute(t *testing.T) {
	registry := NewRegistry()
	registry.Add(domain.ProxyRoute{Target: "127.0.0.1:3010"})

	if len(registry.List()) != 0 {
		t.Error("a route with no host names nothing and must not be stored")
	}
}

func TestRegistryTakesConcurrentWriters(t *testing.T) {
	registry := NewRegistry()

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			host := fmt.Sprintf("job-%d.feat.myapp.localhost", i)
			registry.Add(route(host, fmt.Sprintf("127.0.0.1:%d", 3000+i)))
			registry.Lookup(host)
			registry.List()
		}()
	}
	wg.Wait()

	if len(registry.List()) != 100 {
		t.Errorf("List holds %d routes, want 100", len(registry.List()))
	}
}

// A route survives its job being restarted under the same name: the second Add
// replaces the first rather than leaving a stale target behind.
func TestRegistryAddReplacesAStaleTarget(t *testing.T) {
	registry := NewRegistry()
	registry.Add(route("web.feat.myapp.localhost", "localhost:3010"))
	registry.Add(route("web.feat.myapp.localhost", "localhost:3020"))

	found, _ := registry.Lookup("web.feat.myapp.localhost")
	if found.Target != "localhost:3020" {
		t.Errorf("Target = %q, want the port the job most recently bound", found.Target)
	}
	if len(registry.List()) != 1 {
		t.Errorf("List = %v, want one entry per host", registry.List())
	}
}

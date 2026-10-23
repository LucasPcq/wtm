package registry_test

import (
	"github.com/LucasPcq/wtm/next/internal/adapter/registrytest"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/registry"
)

// service is the registry over store and d, announcing on b.
func service(store *registrytest.Fake, d disk, b *bus) registry.Service {
	return registry.Service{Store: store, Announce: b.announce, Moved: d.isMoved, Initialized: d.initialized}
}

// initialized is the disk where each of repos holds a wtm config in its common directory.
func initialized(repos ...domain.Repo) disk {
	configs := map[string]bool{}
	for _, repo := range repos {
		configs[repo.CommonDir] = true
	}
	return disk{configs: configs}
}

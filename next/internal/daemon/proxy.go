package daemon

import (
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// Proxy returns the proxy the daemon serves named URLs on, as it stands now:
// none when it is off.
func (d *Daemon) Proxy() run.Proxy {
	if d.params.Routes == nil {
		return run.Proxy{}
	}
	port := d.params.Routes.Port()
	if port == 0 {
		return run.Proxy{}
	}
	probed := 0
	if d.params.Probe != nil {
		probed = d.params.Probe(run.PrivilegedPort)
	}

	return run.Proxy{Port: port, PublicPort: run.PublicPort(port, probed)}
}

// serveLocked has the proxy serve the names of j, a service up: a claim has
// none of its own, its shared job serves them. The daemon's lock is held: a
// stop never runs between a job found up and its names served.
func (d *Daemon) serveLocked(j *job) {
	if d.params.Routes == nil || !j.serves() {
		return
	}
	addresses := j.record.Spec.Addresses()
	for _, address := range addresses {
		d.params.Routes.Add(address)
	}
	j.served = len(addresses) > 0
}

// withdrawLocked has the proxy stop serving the names of j, a job no longer
// up; the daemon's lock is held.
func (d *Daemon) withdrawLocked(j *job) {
	if !j.served {
		return
	}
	for _, address := range j.record.Spec.Addresses() {
		d.params.Routes.Remove(address.Host)
	}
	j.served = false
}

// serves reports whether j is a service up the proxy serves names of; the
// daemon's lock is held, or j is not yet in its table.
func (j *job) serves() bool {
	return j.up() && !j.record.Claim && j.record.Spec.Job.Kind == project.Service && len(j.record.Spec.Routes) > 0
}

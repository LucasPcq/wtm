package main

// Every other exported function or method of the four packages is a reader
// (docs/dev/services.md).
var engineMutators = map[string]bool{
	"gitwt.Add":         true,
	"gitwt.Remove":      true,
	"identity.Allocate": true,
	"hooks.Execute":     true,
	"hooks.RunHooks":    true,
	"runenv.Apply":      true,
}

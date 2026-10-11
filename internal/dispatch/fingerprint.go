package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Fingerprint is the hash of the facts a run read: a preview returns it, and
// a run handed it back is refused once the facts no longer match.
type Fingerprint string

const fingerprintBytes = 12

// fingerprintOf hashes the facts as JSON, whose maps are written in key order:
// the same facts always give the same fingerprint. Only what the facts export
// to JSON counts, which is what a command declares it read.
func fingerprintOf(facts any) Fingerprint {
	data, err := json.Marshal(facts)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return Fingerprint(hex.EncodeToString(sum[:fingerprintBytes]))
}

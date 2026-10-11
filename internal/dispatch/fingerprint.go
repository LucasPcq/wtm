package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type Fingerprint string

const fingerprintBytes = 12

// fingerprintOf hashes the facts as JSON, whose maps come out in key order, so
// the same facts always give the same fingerprint.
func fingerprintOf(facts any) Fingerprint {
	data, err := json.Marshal(facts)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return Fingerprint(hex.EncodeToString(sum[:fingerprintBytes]))
}

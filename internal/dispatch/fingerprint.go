package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type Fingerprint string

const fingerprintBytes = 12

// fingerprintOf hashes the facts as JSON, whose maps come out in key order, so
// the same facts always give the same fingerprint. Facts that do not encode
// are an error of the command: a run on them could not be guarded.
func fingerprintOf(facts any) (Fingerprint, error) {
	data, err := json.Marshal(facts)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return Fingerprint(hex.EncodeToString(sum[:fingerprintBytes])), nil
}

package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type Fingerprint string

const fingerprintBytes = 12

func fingerprintOf(facts any) (Fingerprint, error) {
	data, err := json.Marshal(facts)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return Fingerprint(hex.EncodeToString(sum[:fingerprintBytes])), nil
}

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const fingerprintBytes = 12

func fingerprintOf(plan any) (string, error) {
	data, err := json.Marshal(plan)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:fingerprintBytes]), nil
}

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"

	"github.com/LucasPcq/wtm/next/internal/app/create"
)

const fingerprintBytes = 12

type fingerprinted struct {
	Plan      create.Planned    `json:"plan"`
	Isolation project.Isolation `json:"isolation"`
}

func fingerprintOf(applied fingerprinted) (string, error) {
	data, err := json.Marshal(applied)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:fingerprintBytes]), nil
}

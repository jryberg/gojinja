package environment

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
)

// sha256New returns a fresh sha256 hasher. Wrapped in a tiny indirection
// so the call site in environment.go reads cleanly.
func sha256New() hash.Hash { return sha256.New() }

// hexEncode is a thin wrapper for hex.EncodeToString.
func hexEncode(b []byte) string { return hex.EncodeToString(b) }

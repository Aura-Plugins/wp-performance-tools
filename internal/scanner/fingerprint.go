package scanner

import (
	"crypto/sha256"
	"encoding/hex"
)

// Fingerprint returns a stable SHA256 of the fields that identify a finding,
// so the same issue on the same target produces the same value across runs.
// Useful for comparing a scan before and after a fix.
func Fingerprint(f Finding) string {
	sum := sha256.Sum256([]byte(f.ID + "|" + f.Target + "|" + f.Path))
	return hex.EncodeToString(sum[:])
}

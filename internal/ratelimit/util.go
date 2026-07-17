package ratelimit

import (
	"crypto/rand"
	"encoding/hex"
)

// toInt64 normalizes the numeric types go-redis can hand back from an EVAL
// result (int64 from RESP2, or occasionally other numeric types) into int64.
func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

// randSuffix returns a short random hex string used to disambiguate sorted
// set members that share the same millisecond timestamp.
func randSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

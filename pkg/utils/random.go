package utils

import (
	"math/rand"
	"sync"
	"time"
)

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))
var seededRandMu sync.Mutex

const CharsetAlphaLowercase = "abcdefghijklmnopqrstuvwxyz"
const CharsetAlphaNumeric = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const CharsetLowercaseNumeric = "abcdefghijklmnopqrstuvwxyz0123456789"

// Returns a random string of given length using charset
func RandomString(length int, charset string) string {
	b := make([]byte, length)
	lc := len(charset)
	for i := range b {
		seededRandMu.Lock()
		r := seededRand.Intn(lc)
		seededRandMu.Unlock()
		b[i] = charset[r]
	}
	return string(b)
}

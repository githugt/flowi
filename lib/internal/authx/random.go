// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"crypto/rand"
	"math/big"
)

const nonceAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// randomString is n random letters and digits from crypto/rand.
func randomString(n int) string {
	out := make([]byte, n)
	limit := big.NewInt(int64(len(nonceAlphabet)))
	for i := range out {
		v, err := rand.Int(rand.Reader, limit)
		if err != nil {
			panic("authx: no randomness: " + err.Error()) // an unrecoverable system failure; nothing safe can be signed without it
		}
		out[i] = nonceAlphabet[v.Int64()]
	}
	return string(out)
}

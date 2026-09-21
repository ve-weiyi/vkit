package cryptox

import (
	"crypto/sha256"
	"encoding/hex"
)

// Sha256v sha256 加盐哈希
func Sha256v(str string, salt string) string {
	h := sha256.New()
	h.Write([]byte(str + salt))
	var res []byte
	res = h.Sum(nil)
	return hex.EncodeToString(res)
}

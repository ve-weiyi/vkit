package netease

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"math/rand"
	"net/url"
	"strings"
)

func fromData(data map[string]string) io.Reader {
	form := url.Values{}
	for k, v := range data {
		form.Set(k, v)
	}
	return strings.NewReader(form.Encode())
}

func randRange(min, max int64) int64 {
	diff := max - min
	move := rand.Int63n(diff)
	randNum := min + move
	return randNum
}

func long2ip(ip int64) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		(ip>>24)&0xFF,
		(ip>>16)&0xFF,
		(ip>>8)&0xFF,
		ip&0xFF)
}

func randomIP() string {
	return long2ip(randRange(1884815360, 1884890111))
}

// randomBytes 从 charset 中取 length 个随机字符，用于生成 weapi 的会话密钥。
//
// 必须用 crypto/rand：这个密钥是客户端自选的 AES key，用它加密请求体，
// 用 math/rand（时间做种子）生成等于把密钥交给任何能观察到请求的一方。
func randomBytes(length int, charset string) ([]byte, error) {
	if len(charset) == 0 {
		return nil, fmt.Errorf("charset cannot be empty")
	}

	max := big.NewInt(int64(len(charset)))
	b := make([]byte, length)
	for i := range b {
		idx, err := crand.Int(crand.Reader, max)
		if err != nil {
			return nil, fmt.Errorf("generate random bytes: %w", err)
		}
		b[i] = charset[idx.Int64()]
	}

	return b, nil
}

func reverseBytes(src []byte) []byte {
	buf := make([]byte, len(src))
	copy(buf, src)

	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return buf
}

func hexBytes(src []byte) []byte {
	dst := make([]byte, hex.EncodedLen(len(src)))
	hex.Encode(dst, src)
	return dst
}

func base64Bytes(src []byte) []byte {
	buf := make([]byte, base64.StdEncoding.EncodedLen(len(src)))
	base64.StdEncoding.Encode(buf, src)
	return buf
}

func aesEncrypt(text []byte, key []byte, iv []byte) []byte {
	block, _ := aes.NewCipher(key)
	blockSize := block.BlockSize()

	padding := (blockSize - len(text)%blockSize)
	paddingText := append(text, bytes.Repeat([]byte{byte(padding)}, padding)...)

	ciphertext := make([]byte, len(paddingText))

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, paddingText)
	return ciphertext
}

func rsaEncrypt(text []byte, pk []byte) []byte {
	text = append(make([]byte, 128-len(text)), text...)

	block, _ := pem.Decode(pk)
	pub, _ := x509.ParsePKIXPublicKey(block.Bytes)

	pubKey := pub.(*rsa.PublicKey)
	ciphertext := new(big.Int)
	return ciphertext.Exp(ciphertext.SetBytes(text), big.NewInt(int64(pubKey.E)), pubKey.N).Bytes()
}

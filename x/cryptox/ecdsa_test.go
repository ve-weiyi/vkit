package cryptox_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/ve-weiyi/vkit/x/cryptox"
)

func TestEcdsaSignVerify(t *testing.T) {
	privateKey, publicKey, err := cryptox.EcdsaGenerateKey()
	if err != nil {
		t.Fatalf("EcdsaGenerateKey: %v", err)
	}

	hash := sha256.Sum256([]byte("hello ecdsa"))
	hashHex := hex.EncodeToString(hash[:])

	signature, err := cryptox.EcdsaGenerateSignature(privateKey, hashHex)
	if err != nil {
		t.Fatalf("EcdsaGenerateSignature: %v", err)
	}

	valid, err := cryptox.EcdsaVerifySignature(publicKey, hashHex, signature)
	if err != nil {
		t.Fatalf("EcdsaVerifySignature: %v", err)
	}
	if !valid {
		t.Fatal("正确签名应校验通过")
	}

	otherHash := sha256.Sum256([]byte("tampered"))
	valid, err = cryptox.EcdsaVerifySignature(publicKey, hex.EncodeToString(otherHash[:]), signature)
	if err != nil {
		t.Fatalf("EcdsaVerifySignature: %v", err)
	}
	if valid {
		t.Error("被篡改的哈希不应校验通过")
	}
}

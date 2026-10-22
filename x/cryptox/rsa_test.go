package cryptox_test

import (
	"crypto/x509"
	"encoding/base64"
	"testing"

	"github.com/ve-weiyi/vkit/x/cryptox"
)

func TestGenerateRsaKeys(t *testing.T) {
	publicKey, privateKey, err := cryptox.GenerateRsaKeys()
	if err != nil {
		t.Fatalf("GenerateRsaKeys: %v", err)
	}

	publicBytes, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		t.Fatalf("公钥不是合法 base64: %v", err)
	}
	if _, err := x509.ParsePKIXPublicKey(publicBytes); err != nil {
		t.Errorf("公钥无法解析: %v", err)
	}

	privateBytes, err := base64.StdEncoding.DecodeString(privateKey)
	if err != nil {
		t.Fatalf("私钥不是合法 base64: %v", err)
	}
	if _, err := x509.ParsePKCS1PrivateKey(privateBytes); err != nil {
		t.Errorf("私钥无法解析: %v", err)
	}
}

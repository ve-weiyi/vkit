package cryptox

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
)

const (
	// RSA_KEY_SIZE 密钥位数，2048 是现代最低强度（1024 已被认为不安全）
	RSA_KEY_SIZE = 2048
)

func base64PrivateKey(privateKey *rsa.PrivateKey) string {
	privateBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	return base64.StdEncoding.EncodeToString(privateBytes)
}
func base64PublicKey(publicKey *rsa.PublicKey) (string, error) {
	publicBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(publicBytes), nil
}

func GenerateRsaKeys() (publicKey string, privateKey string, err error) {
	key, err := rsa.GenerateKey(rand.Reader, RSA_KEY_SIZE)
	if err != nil {
		return "", "", err
	}

	publicKey, err = base64PublicKey(&(key.PublicKey))
	if err != nil {
		return "", "", err
	}

	return publicKey, base64PrivateKey(key), nil
}

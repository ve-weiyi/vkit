package cryptox_test

import (
	"bytes"
	"testing"

	"github.com/ve-weiyi/vkit/x/cryptox"
)

// 三种模式都必须能完成加解密往返，且密文与明文不同。
func TestAESRoundTrip(t *testing.T) {
	key, err := cryptox.GenerateRandomAESKey(32)
	if err != nil {
		t.Fatalf("GenerateRandomAESKey: %v", err)
	}

	plaintext := []byte("Hello, World! This is a test message for AES encryption.")

	cases := []struct {
		name   string
		cipher cryptox.AES
		iv     []byte
	}{
		{"CBC", cryptox.NewCBC(), bytes.Repeat([]byte{1}, 16)},
		{"CFB", cryptox.NewCFB(), bytes.Repeat([]byte{1}, 16)},
		{"GCM", cryptox.NewGCM(), bytes.Repeat([]byte{1}, 12)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, err := tc.cipher.AESEncrypt(plaintext, key, tc.iv...)
			if err != nil {
				t.Fatalf("AESEncrypt: %v", err)
			}
			if bytes.Equal(ciphertext, plaintext) {
				t.Fatal("密文不应等于明文")
			}

			decrypted, err := tc.cipher.AESDecrypt(ciphertext, key, tc.iv...)
			if err != nil {
				t.Fatalf("AESDecrypt: %v", err)
			}
			if !bytes.Equal(decrypted, plaintext) {
				t.Errorf("解密结果 = %q, want %q", decrypted, plaintext)
			}
		})
	}
}

func TestGenerateRandomAESKey(t *testing.T) {
	for _, size := range []int{16, 24, 32} {
		key, err := cryptox.GenerateRandomAESKey(size)
		if err != nil {
			t.Fatalf("GenerateRandomAESKey(%d): %v", size, err)
		}
		if len(key) != size {
			t.Errorf("密钥长度 = %d, want %d", len(key), size)
		}
	}

	if _, err := cryptox.GenerateRandomAESKey(20); err == nil {
		t.Error("非法密钥长度应返回错误")
	}
}

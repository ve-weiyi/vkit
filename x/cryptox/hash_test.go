package cryptox_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ve-weiyi/vkit/x/cryptox"
)

func TestHash(t *testing.T) {
	cases := []struct {
		name string
		fn   func(string, string) string
	}{
		{"Md5v", cryptox.Md5v},
		{"Sha256v", cryptox.Sha256v},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.fn("dev", "salt")
			if got != tc.fn("dev", "salt") {
				t.Errorf("%s 对相同输入结果不稳定", tc.name)
			}
			if got == tc.fn("dev", "other-salt") {
				t.Errorf("%s 换盐后结果不应相同", tc.name)
			}
			if _, err := hex.DecodeString(got); err != nil {
				t.Errorf("%s 结果不是合法 hex: %v", tc.name, err)
			}
		})
	}
}

func TestBcrypt(t *testing.T) {
	const password = "correct-horse-battery-staple"

	ciphertext, err := cryptox.BcryptHash(password)
	if err != nil {
		t.Fatalf("BcryptHash: %v", err)
	}
	if !cryptox.BcryptCheck(password, ciphertext) {
		t.Error("BcryptCheck(正确密码) 应为 true")
	}
	if cryptox.BcryptCheck("wrong-password", ciphertext) {
		t.Error("BcryptCheck(错误密码) 应为 false")
	}

	// 超过 72 字节必须报错，否则写库后该用户永远无法登录
	if _, err := cryptox.BcryptHash(strings.Repeat("a", 100)); err == nil {
		t.Error("超过 72 字节的密码应返回错误")
	}
}

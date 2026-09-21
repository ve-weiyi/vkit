package jwtx

import (
	"testing"
	"time"
)

func Test_JwtInstance_GenerateJWT(t *testing.T) {
	jt := NewJwtInstance([]byte("2024/3/23"))

	token, err := jt.CreateToken(
		WithIssuer("test"),
		WithSubject("test"),
		WithAudience("test"),
		WithExpiresAt(time.Now().Unix()+3600),
		WithNotBefore(time.Now().Unix()),
		WithIssuedAt(time.Now().Unix()),
		WithId("test"),
		WithClaimExt("test", "test"),
	)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if token == "" {
		t.Fatal("CreateToken 返回空 token")
	}

	tk, err := jt.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if tk == nil || !tk.Valid {
		t.Fatal("解析出的 token 应为有效")
	}

	// 换一把密钥应当解析失败
	other := NewJwtInstance([]byte("another-key"))
	if _, err := other.ParseToken(token); err == nil {
		t.Error("用不同密钥解析应失败")
	}
}

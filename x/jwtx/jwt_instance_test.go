package jwtx

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("0123456789abcdef0123456789abcdef")

func TestJwtInstanceRoundTrip(t *testing.T) {
	jt := NewJwtInstance(testSecret)

	token, err := jt.CreateToken(
		WithIssuer("test"),
		WithSubject("test"),
		WithAudience("test"),
		WithExpiresAt(time.Now().Add(time.Hour).Unix()),
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

	// 换一把 32 字节密钥应当解析失败
	other := NewJwtInstance([]byte("fedcba9876543210fedcba9876543210"))
	if _, err := other.ParseToken(token); err == nil {
		t.Error("用不同密钥解析应失败")
	}
}

func TestCreateTokenRejectsShortSecret(t *testing.T) {
	jt := NewJwtInstance([]byte("short"))
	if _, err := jt.CreateToken(WithExpiresAt(time.Now().Add(time.Hour).Unix())); err == nil {
		t.Fatal("短密钥应被拒绝")
	}
}

func TestParseTokenRejectsShortSecret(t *testing.T) {
	jt := NewJwtInstance([]byte("short"))
	if _, err := jt.ParseToken("any-token"); err == nil {
		t.Fatal("短密钥应被拒绝")
	}
}

func TestCreateTokenRequiresExpiration(t *testing.T) {
	jt := NewJwtInstance(testSecret)
	if _, err := jt.CreateToken(WithSubject("test")); err == nil {
		t.Fatal("缺少 exp 应被拒绝")
	}
}

func TestParseTokenRequiresExpiration(t *testing.T) {
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "test",
	}).SignedString(testSecret)
	if err != nil {
		t.Fatal(err)
	}

	jt := NewJwtInstance(testSecret)
	if _, err := jt.ParseToken(raw); err == nil {
		t.Fatal("缺少 exp 的 token 应被拒绝")
	}
}

func TestParseTokenRejectsExpired(t *testing.T) {
	jt := NewJwtInstance(testSecret)
	token, err := jt.CreateToken(WithExpiresAt(time.Now().Add(-time.Hour).Unix()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jt.ParseToken(token); err == nil {
		t.Fatal("过期 token 应被拒绝")
	}
}

func TestParseTokenRejectsNoneAlgorithm(t *testing.T) {
	raw, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}

	jt := NewJwtInstance(testSecret)
	if _, err := jt.ParseToken(raw); err == nil {
		t.Fatal("alg=none 应被拒绝")
	}
}

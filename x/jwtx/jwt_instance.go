package jwtx

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	JwtAudience  = "aud"
	JwtExpire    = "exp"
	JwtId        = "jti"
	JwtIssueAt   = "iat"
	JwtIssuer    = "iss"
	JwtNotBefore = "nbf"
	JwtSubject   = "sub"
	JwtExt       = "ext"
)

const (
	// minSecretKeySize 是 HS256 的最小密钥长度。短于 32 字节的密钥强度不足，
	// 与 tokenx 的 jwtcodec 保持一致。
	minSecretKeySize = 32

	// parseLeeway 容忍少量时钟偏差，避免多副本之间因时钟漂移误判过期。
	parseLeeway = 30 * time.Second
)

type JwtInstance struct {
	SecretKey []byte
}

func NewJwtInstance(SecretKey []byte) *JwtInstance {
	return &JwtInstance{SecretKey}
}

// CreateToken 生成JWT。
//
// exp 是必需声明：没有过期时间的令牌一旦泄露就是永久凭据。
func (that JwtInstance) CreateToken(options ...Option) (string, error) {
	if len(that.SecretKey) < minSecretKeySize {
		return "", fmt.Errorf("jwtx: secret key must be at least %d bytes", minSecretKeySize)
	}

	claims := jwt.MapClaims{}

	for _, option := range options {
		option(claims)
	}

	if _, ok := claims[JwtExpire]; !ok {
		return "", errors.New("jwtx: exp claim is required")
	}

	// 创建一个新的JWT token
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 设置签名并获取token字符串
	token, err := jwtToken.SignedString(that.SecretKey)
	if err != nil {
		return "", err
	}
	return token, nil
}

// ParseToken 解析并校验JWT。
//
// 只接受 HS256：算法白名单防止 alg 混淆；exp 必填，避免无期限令牌。
func (that JwtInstance) ParseToken(tokenString string) (*jwt.Token, error) {
	if len(that.SecretKey) < minSecretKeySize {
		return nil, fmt.Errorf("jwtx: secret key must be at least %d bytes", minSecretKeySize)
	}

	// 解析JWT字符串
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return that.SecretKey, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(parseLeeway),
	)

	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("jwtx: invalid token")
	}

	return token, nil
	// 验证token
	//if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
	//	return claims, nil
	//}
	//
	//return nil, fmt.Errorf("invalid token")
}

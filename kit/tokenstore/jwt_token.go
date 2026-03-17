package tokenstore

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/ve-weiyi/pkg/utils/jwtx"
)

// JwtTokenStore JWT Token 管理器实现，支持单设备登录
type JwtTokenStore struct {
	store             TokenCache
	jwtInstance       *jwtx.JwtInstance
	issuer            string
	accessExpireTime  int64 // 秒
	refreshExpireTime int64 // 秒
}

// NewJwtTokenStore 创建 JWT Token 管理器
func NewJwtTokenStore(store TokenCache, secretKey, issuer string, accessExpire, refreshExpire int64) *JwtTokenStore {
	return &JwtTokenStore{
		store:             store,
		jwtInstance:       jwtx.NewJwtInstance([]byte(secretKey)),
		issuer:            issuer,
		accessExpireTime:  accessExpire,
		refreshExpireTime: refreshExpire,
	}
}

// GenerateToken 生成 JWT Token
func (m *JwtTokenStore) GenerateToken(uid string) (*Token, error) {
	if uid == "" {
		return nil, fmt.Errorf("uid is empty")
	}
	now := time.Now().Unix()

	// 生成 AccessToken
	accessToken, err := m.jwtInstance.CreateToken(
		jwtx.WithSubject(uid),
		jwtx.WithIssuer(m.issuer),
		jwtx.WithIssuedAt(now),
		jwtx.WithExpiresAt(now+m.accessExpireTime),
	)
	if err != nil {
		return nil, err
	}

	// 生成 RefreshToken
	refreshToken, err := m.jwtInstance.CreateToken(
		jwtx.WithSubject(uid),
		jwtx.WithIssuer(m.issuer),
		jwtx.WithIssuedAt(now),
		jwtx.WithExpiresAt(now+m.refreshExpireTime),
	)
	if err != nil {
		return nil, err
	}

	// 分开存储 AccessToken 和 RefreshToken
	if err := m.store.Set(fmt.Sprintf("%s:%s", TokenPrefixAccess, uid), accessToken, int(m.accessExpireTime)); err != nil {
		return nil, err
	}
	if err := m.store.Set(fmt.Sprintf("%s:%s", TokenPrefixRefresh, uid), refreshToken, int(m.refreshExpireTime)); err != nil {
		return nil, err
	}

	return &Token{
		TokenType:        TokenTypeBearer,
		AccessToken:      accessToken,
		ExpiresIn:        m.accessExpireTime,
		RefreshToken:     refreshToken,
		RefreshExpiresIn: m.refreshExpireTime,
		RefreshExpiresAt: now + m.refreshExpireTime,
	}, nil
}

// ValidateToken 验证 AccessToken 有效性
func (m *JwtTokenStore) ValidateToken(uid, accessToken string) error {
	_, err := m.jwtInstance.ParseToken(accessToken)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return ErrTokenExpired
		}
		return ErrTokenInvalid
	}

	// 检查存储中的 AccessToken 是否匹配
	storedToken, err := m.store.Get(fmt.Sprintf("%s:%s", TokenPrefixAccess, uid))
	if err != nil {
		return err
	}
	if storedToken == "" {
		return ErrTokenExpired
	}
	if storedToken != accessToken {
		return ErrTokenInvalid
	}

	return nil
}

// RefreshToken 使用 RefreshToken 刷新获取新 Token
func (m *JwtTokenStore) RefreshToken(uid, refreshToken string) (*Token, error) {
	_, err := m.jwtInstance.ParseToken(refreshToken)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	// 检查存储中的 RefreshToken 是否匹配
	storedToken, err := m.store.Get(fmt.Sprintf("%s:%s", TokenPrefixRefresh, uid))
	if err != nil || storedToken == "" {
		return nil, ErrTokenExpired
	}
	if storedToken != refreshToken {
		return nil, ErrTokenInvalid
	}

	// 生成新的 Token
	return m.GenerateToken(uid)
}

// RevokeToken 撤销 Token
func (m *JwtTokenStore) RevokeToken(uid string, isRefresh bool) error {
	if isRefresh {
		return m.store.Delete(fmt.Sprintf("%s:%s", TokenPrefixRefresh, uid))
	}
	return m.store.Delete(fmt.Sprintf("%s:%s", TokenPrefixAccess, uid))
}

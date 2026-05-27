package tokenstore

import (
	"context"
	"crypto/md5"
	"fmt"
	"time"

	"github.com/ve-weiyi/vkit/adapter/storex"
)

// SignTokenStore 基于签名的 Token 管理器实现，支持单设备登录
type SignTokenStore struct {
	store             storex.KVStore
	secretKey         string
	issuer            string
	accessExpireTime  int64
	refreshExpireTime int64
}

// NewSignTokenStore 创建签名 Token 管理器
func NewSignTokenStore(store storex.KVStore, secretKey, issuer string, accessExpire, refreshExpire int64) *SignTokenStore {
	return &SignTokenStore{
		store:             store,
		secretKey:         secretKey,
		issuer:            issuer,
		accessExpireTime:  accessExpire,
		refreshExpireTime: refreshExpire,
	}
}

// GenerateToken 生成签名 Token
func (m *SignTokenStore) GenerateToken(uid string) (*Token, error) {
	if uid == "" {
		return nil, fmt.Errorf("uid is empty")
	}
	ctx := context.Background()
	accessToken := m.sign(uid)
	refreshToken := m.sign(uid)

	// 分开存储 AccessToken 和 RefreshToken
	if err := m.store.Set(ctx, fmt.Sprintf("%s:%s", TokenPrefixAccess, uid), accessToken, time.Duration(m.accessExpireTime)*time.Second); err != nil {
		return nil, err
	}
	if err := m.store.Set(ctx, fmt.Sprintf("%s:%s", TokenPrefixRefresh, uid), refreshToken, time.Duration(m.refreshExpireTime)*time.Second); err != nil {
		return nil, err
	}

	return &Token{
		TokenType:        TokenTypeSign,
		AccessToken:      accessToken,
		ExpiresIn:        m.accessExpireTime,
		RefreshToken:     refreshToken,
		RefreshExpiresIn: m.refreshExpireTime,
		RefreshExpiresAt: time.Now().Unix() + int64(m.refreshExpireTime),
	}, nil
}

// ValidateToken 验证 Token 有效性
func (m *SignTokenStore) ValidateToken(uid, accessToken string) error {
	ctx := context.Background()
	storedToken, err := m.store.Get(ctx, fmt.Sprintf("%s:%s", TokenPrefixAccess, uid))
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

// RefreshToken 刷新 Token
func (m *SignTokenStore) RefreshToken(uid, refreshToken string) (*Token, error) {
	ctx := context.Background()
	storedToken, err := m.store.Get(ctx, fmt.Sprintf("%s:%s", TokenPrefixRefresh, uid))
	if err != nil || storedToken == "" {
		return nil, ErrTokenExpired
	}
	if storedToken != refreshToken {
		return nil, ErrTokenInvalid
	}

	return m.GenerateToken(uid)
}

// RevokeToken 撤销 Token
func (m *SignTokenStore) RevokeToken(uid string, isRefresh bool) error {
	ctx := context.Background()
	if isRefresh {
		return m.store.Delete(ctx, fmt.Sprintf("%s:%s", TokenPrefixRefresh, uid))
	}
	return m.store.Delete(ctx, fmt.Sprintf("%s:%s", TokenPrefixAccess, uid))
}

// sign 生成签名token: MD5(uid + timestamp + issuer + secret)
func (m *SignTokenStore) sign(uid string) string {
	timestamp := time.Now().UnixMilli()
	return fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%d:%s:%s", uid, timestamp, m.issuer, m.secretKey))))
}

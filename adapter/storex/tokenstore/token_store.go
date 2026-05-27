package tokenstore

import (
	"fmt"
)

const (
	TokenTypeBearer = "Bearer" // 使用jwt
	TokenTypeSign   = "Sign"   // 使用md5

	// Token存储前缀
	TokenPrefixAccess  = "access"  // AccessToken前缀
	TokenPrefixRefresh = "refresh" // RefreshToken前缀
)

var (
	ErrTokenEmpty   = fmt.Errorf("token is empty")
	ErrTokenInvalid = fmt.Errorf("token is invalid")
	ErrTokenExpired = fmt.Errorf("token is expired")
)

// Token 结构体定义 Token 相关的核心字段，适配不同 Token 实现的统一返回格式
type Token struct {
	TokenType        string `json:"token_type"`         // Token 类型（如 "Bearer"）
	AccessToken      string `json:"access_token"`       // 访问令牌：用于接口访问，有效期短
	ExpiresIn        int64  `json:"expires_in"`         // AccessToken 有效期（秒），如 3600（1小时）
	RefreshToken     string `json:"refresh_token"`      // 刷新令牌：仅用于刷新 AccessToken，有效期长
	RefreshExpiresIn int64  `json:"refresh_expires_in"` // RefreshToken 有效期（秒），如 604800（7天）
	RefreshExpiresAt int64  `json:"refresh_expires_at"` // RefreshToken 过期时间戳（秒）
}

// TokenStore 定义 Token 全生命周期管理的核心接口
type TokenStore interface {
	// GenerateToken 基于用户唯一标识 uid 生成 Token 结构体
	GenerateToken(uid string) (*Token, error)

	// ValidateToken 验证 AccessToken 的有效性
	ValidateToken(uid, accessToken string) error

	// RefreshToken 使用旧的 RefreshToken 刷新获取新的 Token 结构体
	RefreshToken(uid, refreshToken string) (*Token, error)

	// RevokeToken 主动作废 Token
	RevokeToken(uid string, isRefresh bool) error
}

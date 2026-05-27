package smsx

import (
	"context"
	"time"
)

// SmsProvider 短信服务提供商接口
// 支持多种短信服务商：阿里云、腾讯云、Mock等
type SmsProvider interface {
	// SendCode 发送验证码短信
	// phone: 手机号
	// code: 验证码
	// codeType: 验证码类型（login-登录 register-注册 reset_password-重置密码）
	// expireTime: 过期时间（分钟）
	SendCode(ctx context.Context, phone, codeType, code string, expireTime int) error

	// SendTemplate 发送模板短信
	// phone: 手机号
	// templateCode: 模板代码
	// params: 模板参数
	SendTemplate(ctx context.Context, phone, templateCode string, params map[string]string) error

	// GetTemplateCode 根据场景获取模板代码
	// scene: 场景（register, login, reset_password等）
	GetTemplateCode(scene string) string

	// GetProviderName 获取服务商名称
	GetProviderName() string
}

// SmsCodeRecord 短信验证码记录
type SmsCodeRecord struct {
	Phone     string
	Code      string
	CodeType  string
	IPAddress string
	ExpiredAt time.Time
	CreatedAt time.Time
}

// SmsConfig 短信服务配置
type SmsConfig struct {
	Provider  string            // 服务商类型：aliyun | tencent | mock
	AccessKey string            // AccessKey / SecretId
	SecretKey string            // SecretKey
	SignName  string            // 短信签名
	Region    string            // 地域（腾讯云需要）
	SdkAppId  string            // SDK应用ID（腾讯云需要）
	Templates map[string]string // 模板配置：codeType -> templateCode/templateId
}

// NewSmsProvider 创建短信服务提供商实例（工厂模式）
func NewSmsProvider(config *SmsConfig) SmsProvider {
	switch config.Provider {
	case "aliyun":
		return NewAliyunSmsProvider(config)
	case "tencent":
		return NewTencentSmsProvider(config)
	case "mock":
		return NewMockSmsProvider(config)
	default:
		// 默认使用 Mock 提供商（开发环境）
		return NewMockSmsProvider(config)
	}
}

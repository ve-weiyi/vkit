package smsx

import (
	"context"
	"fmt"
)

// MockSmsProvider Mock短信服务提供商（用于开发和测试环境）
type MockSmsProvider struct {
	config *SmsConfig
}

// NewMockSmsProvider 创建Mock短信服务提供商实例
func NewMockSmsProvider(config *SmsConfig) *MockSmsProvider {
	return &MockSmsProvider{
		config: config,
	}
}

// SendCode 发送验证码短信（Mock实现，仅打印日志）
func (p *MockSmsProvider) SendCode(ctx context.Context, phone, codeType, code string, expireTime int) error {
	fmt.Printf("[MockSms] SendCode - Phone: %s, Code: %s, Type: %s, ExpireTime: %d minutes\n", phone, code, codeType, expireTime)
	return nil
}

// SendTemplate 发送模板短信（Mock实现，仅打印日志）
func (p *MockSmsProvider) SendTemplate(ctx context.Context, phone, templateCode string, params map[string]string) error {
	fmt.Printf("[MockSms] SendTemplate - Phone: %s, Template: %s, Params: %v\n", phone, templateCode, params)
	return nil
}

// GetProviderName 获取服务商名称
func (p *MockSmsProvider) GetProviderName() string {
	return "mock"
}

// GetTemplateCode 根据场景获取模板代码
func (p *MockSmsProvider) GetTemplateCode(scene string) string {
	return "MOCK_" + scene
}

package smsx

import (
	"context"
	"testing"
)

// TestMockSmsProvider 测试 Mock 短信服务提供商
func TestMockSmsProvider(t *testing.T) {
	config := &SmsConfig{
		Provider: "mock",
		SignName: "测试签名",
	}

	provider, err := NewSmsProvider(config)
	if err != nil {
		t.Fatalf("NewSmsProvider: %v", err)
	}

	if provider.GetProviderName() != "mock" {
		t.Errorf("Expected provider name 'mock', got '%s'", provider.GetProviderName())
	}

	ctx := context.Background()

	// 测试发送验证码
	err = provider.SendCode(ctx, "13800138000", "login", "123456", 15)
	if err != nil {
		t.Errorf("SendCode failed: %v", err)
	}

	// 测试发送模板短信
	params := map[string]string{
		"code": "654321",
		"time": "5",
	}
	err = provider.SendTemplate(ctx, "13800138000", "SMS_TEST", params)
	if err != nil {
		t.Errorf("SendTemplate failed: %v", err)
	}
}

// TestGetTemplateCode 测试模板映射：配置优先，未配置回落默认，未知场景返回空
func TestGetTemplateCode(t *testing.T) {
	override := map[string]string{"login": "CUSTOM_LOGIN"}

	cases := []struct {
		name   string
		config *SmsConfig
		scene  string
		want   string
	}{
		{"mock 前缀", &SmsConfig{Provider: "mock"}, "login", "MOCK_login"},
		{"aliyun 未配置返回空", &SmsConfig{Provider: "aliyun"}, "login", ""},
		{"aliyun 配置覆盖", &SmsConfig{Provider: "aliyun", Templates: override}, "login", "CUSTOM_LOGIN"},
		{"aliyun 未知场景", &SmsConfig{Provider: "aliyun"}, "unknown", ""},
		{"tencent 未配置返回空", &SmsConfig{Provider: "tencent"}, "register", ""},
		{"tencent 配置覆盖", &SmsConfig{Provider: "tencent", Templates: override}, "login", "CUSTOM_LOGIN"},
		{"tencent 未知场景", &SmsConfig{Provider: "tencent"}, "unknown", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, err := NewSmsProvider(c.config)
			if err != nil {
				t.Fatalf("NewSmsProvider(%s): %v", c.name, err)
			}
			if got := provider.GetTemplateCode(c.scene); got != c.want {
				t.Errorf("GetTemplateCode(%q) = %q, want %q", c.scene, got, c.want)
			}
		})
	}
}

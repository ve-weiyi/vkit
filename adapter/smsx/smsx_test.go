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

	provider := NewSmsProvider(config)
	if provider == nil {
		t.Fatal("Failed to create mock SMS provider")
	}

	if provider.GetProviderName() != "mock" {
		t.Errorf("Expected provider name 'mock', got '%s'", provider.GetProviderName())
	}

	ctx := context.Background()

	// 测试发送验证码
	err := provider.SendCode(ctx, "13959777439", "login", "123456", 15)
	if err != nil {
		t.Errorf("SendCode failed: %v", err)
	}

	// 测试发送模板短信
	params := map[string]string{
		"code": "654321",
		"time": "5",
	}
	err = provider.SendTemplate(ctx, "13959777439", "SMS_TEST", params)
	if err != nil {
		t.Errorf("SendTemplate failed: %v", err)
	}
}

// TestAliyunSmsProvider 测试阿里云短信服务提供商
func TestAliyunSmsProvider(t *testing.T) {
	// 跳过测试，除非设置了真实的凭证

	config := &SmsConfig{
		Provider:  "aliyun",
		AccessKey: "xx",
		SecretKey: "xx",
		SignName:  "xx",
		Templates: map[string]string{
			"code":  "SMS_501860619",
			"login": "SMS_501755620",
		},
	}

	provider := NewSmsProvider(config)
	if provider == nil {
		t.Fatal("Failed to create Aliyun SMS provider")
	}

	ctx := context.Background()
	err := provider.SendCode(ctx, "13959777439", "code", "123456", 15)
	if err != nil {
		t.Errorf("SendCode failed: %v", err)
	}
}

// TestTencentSmsProvider 测试腾讯云短信服务提供商
func TestTencentSmsProvider(t *testing.T) {
	// 跳过测试，除非设置了真实的凭证
	t.Skip("Skipping Tencent SMS test - requires real credentials")

	config := &SmsConfig{
		Provider:  "tencent",
		AccessKey: "your-secret-id",
		SecretKey: "your-secret-key",
		SignName:  "your-sign-name",
		Region:    "ap-guangzhou",
		SdkAppId:  "your-sdk-app-id",
		Templates: map[string]string{
			"login": "1000001",
		},
	}

	provider := NewSmsProvider(config)
	if provider == nil {
		t.Fatal("Failed to create Tencent SMS provider")
	}

	ctx := context.Background()
	err := provider.SendCode(ctx, "13959777439", "login", "123456", 15)
	if err != nil {
		t.Errorf("SendCode failed: %v", err)
	}
}

package smsx

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/dysmsapi"
	"github.com/zeromicro/go-zero/core/logx"
)

// AliyunSmsProvider 阿里云短信服务提供商
type AliyunSmsProvider struct {
	config *SmsConfig
	client *dysmsapi.Client
}

// NewAliyunSmsProvider 创建阿里云短信服务提供商实例
func NewAliyunSmsProvider(config *SmsConfig) *AliyunSmsProvider {
	// 创建阿里云短信客户端
	client, err := dysmsapi.NewClientWithAccessKey(
		"cn-hangzhou", // 默认区域
		config.AccessKey,
		config.SecretKey,
	)
	if err != nil {
		logx.Errorf("Failed to create Aliyun SMS client: %v", err)
		return nil
	}

	return &AliyunSmsProvider{
		config: config,
		client: client,
	}
}

// SendCode 发送验证码短信
func (p *AliyunSmsProvider) SendCode(ctx context.Context, phone, codeType, code string, expireTime int) error {
	// 获取模板代码
	templateCode := p.getTemplateCode(codeType)
	if templateCode == "" {
		return fmt.Errorf("template code not found for type: %s", codeType)
	}

	// 构建模板参数
	params := map[string]string{
		"code": code,
		"time": fmt.Sprintf("%d", expireTime),
	}

	return p.SendTemplate(ctx, phone, templateCode, params)
}

// SendTemplate 发送模板短信
func (p *AliyunSmsProvider) SendTemplate(ctx context.Context, phone, templateCode string, params map[string]string) error {
	// 创建发送请求
	request := dysmsapi.CreateSendSmsRequest()
	request.Scheme = "https"
	request.PhoneNumbers = phone
	request.SignName = p.config.SignName
	request.TemplateCode = templateCode

	// 转换参数为 JSON 字符串
	if len(params) > 0 {
		paramsJSON, err := json.Marshal(params)
		if err != nil {
			logx.Errorf("Failed to marshal template params: %v", err)
			return fmt.Errorf("failed to marshal template params: %w", err)
		}
		request.TemplateParam = string(paramsJSON)
	}

	// 发送短信
	response, err := p.client.SendSms(request)
	if err != nil {
		logx.Errorf("Failed to send SMS via Aliyun: %v", err)
		return fmt.Errorf("failed to send SMS: %w", err)
	}

	// 检查响应
	if response.Code != "OK" {
		logx.Errorf("Aliyun SMS send failed: Code=%s, Message=%s", response.Code, response.Message)
		return fmt.Errorf("SMS send failed: %s - %s", response.Code, response.Message)
	}

	logx.Infof("Aliyun SMS sent successfully: Phone=%s, BizId=%s", phone, response.BizId)
	return nil
}

// GetProviderName 获取服务商名称
func (p *AliyunSmsProvider) GetProviderName() string {
	return "aliyun"
}

// GetTemplateCode 根据场景获取模板代码
func (p *AliyunSmsProvider) GetTemplateCode(scene string) string {
	return p.getTemplateCode(scene)
}

// getTemplateCode 根据验证码类型获取模板代码
func (p *AliyunSmsProvider) getTemplateCode(codeType string) string {
	if p.config.Templates != nil {
		if templateCode, ok := p.config.Templates[codeType]; ok {
			return templateCode
		}
	}

	// 默认模板代码（如果配置中没有指定）
	defaultTemplates := map[string]string{
		"login":          "SMS_LOGIN",
		"register":       "SMS_REGISTER",
		"reset_password": "SMS_RESET_PASSWORD",
		"bind_email":     "SMS_BIND_EMAIL",
		"bind_phone":     "SMS_BIND_PHONE",
	}

	return defaultTemplates[codeType]
}

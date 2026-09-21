package smsx

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/dysmsapi"
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
			return fmt.Errorf("failed to marshal template params: %w", err)
		}
		request.TemplateParam = string(paramsJSON)
	}

	// 发送短信
	response, err := p.client.SendSms(request)
	if err != nil {
		return fmt.Errorf("failed to send SMS: %w", err)
	}

	// 检查响应
	if response.Code != "OK" {
		return fmt.Errorf("SMS send failed: %s - %s", response.Code, response.Message)
	}

	return nil
}

// GetProviderName 获取服务商名称
func (p *AliyunSmsProvider) GetProviderName() string {
	return ProviderAliyun
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

	// 不再内置兜底模板代码：占位值会在未配置时把短信真的发出去（且内容错误），
	// 必须返回空串交由调用方报错，逼迫配置到位
	return ""
}

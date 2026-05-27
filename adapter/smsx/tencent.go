package smsx

import (
	"context"
	"fmt"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
	"github.com/zeromicro/go-zero/core/logx"
)

// TencentSmsProvider 腾讯云短信服务提供商
type TencentSmsProvider struct {
	config *SmsConfig
	client *sms.Client
}

// NewTencentSmsProvider 创建腾讯云短信服务提供商实例
func NewTencentSmsProvider(config *SmsConfig) *TencentSmsProvider {
	// 创建认证对象
	credential := common.NewCredential(
		config.AccessKey, // SecretId
		config.SecretKey, // SecretKey
	)

	// 设置地域，默认为 ap-guangzhou
	region := config.Region
	if region == "" {
		region = "ap-guangzhou"
	}

	// 创建客户端配置
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "sms.tencentcloudapi.com"

	// 创建短信客户端
	client, err := sms.NewClient(credential, region, cpf)
	if err != nil {
		logx.Errorf("Failed to create Tencent SMS client: %v", err)
		return nil
	}

	return &TencentSmsProvider{
		config: config,
		client: client,
	}
}

// SendCode 发送验证码短信
func (p *TencentSmsProvider) SendCode(ctx context.Context, phone, codeType, code string, expireTime int) error {
	// 获取模板 ID
	templateId := p.getTemplateId(codeType)
	if templateId == "" {
		return fmt.Errorf("template ID not found for type: %s", codeType)
	}

	// 构建模板参数（腾讯云使用数组形式）
	params := map[string]string{
		"code": code,
		"time": fmt.Sprintf("%d", expireTime),
	}

	return p.SendTemplate(ctx, phone, templateId, params)
}

// SendTemplate 发送模板短信
func (p *TencentSmsProvider) SendTemplate(ctx context.Context, phone, templateId string, params map[string]string) error {
	// 创建发送请求
	request := sms.NewSendSmsRequest()

	// 设置短信应用 ID
	request.SmsSdkAppId = common.StringPtr(p.config.SdkAppId)

	// 设置短信签名
	request.SignName = common.StringPtr(p.config.SignName)

	// 设置模板 ID
	request.TemplateId = common.StringPtr(templateId)

	// 设置手机号（需要添加国际区号，如 +86）
	phoneNumber := phone
	if phone[0] != '+' {
		phoneNumber = "+86" + phone
	}
	request.PhoneNumberSet = common.StringPtrs([]string{phoneNumber})

	// 设置模板参数（腾讯云使用数组形式，按模板中的顺序）
	// 注意：这里假设模板参数只有一个 code，实际使用时需要根据模板调整
	if len(params) > 0 {
		templateParams := make([]string, 0, len(params))
		// 如果有 code 参数，添加到数组
		if code, ok := params["code"]; ok {
			templateParams = append(templateParams, code)
		}
		// 添加其他参数（按字母顺序）
		for key, value := range params {
			if key != "code" {
				templateParams = append(templateParams, value)
			}
		}
		request.TemplateParamSet = common.StringPtrs(templateParams)
	}

	// 发送短信
	response, err := p.client.SendSms(request)
	if err != nil {
		logx.Errorf("Failed to send SMS via Tencent: %v", err)
		return fmt.Errorf("failed to send SMS: %w", err)
	}

	// 检查响应
	if len(response.Response.SendStatusSet) > 0 {
		status := response.Response.SendStatusSet[0]
		if status.Code == nil || *status.Code != "Ok" {
			errCode := "Unknown"
			errMsg := "Unknown error"
			if status.Code != nil {
				errCode = *status.Code
			}
			if status.Message != nil {
				errMsg = *status.Message
			}
			logx.Errorf("Tencent SMS send failed: Code=%s, Message=%s", errCode, errMsg)
			return fmt.Errorf("SMS send failed: %s - %s", errCode, errMsg)
		}

		// 记录成功日志
		serialNo := "Unknown"
		if status.SerialNo != nil {
			serialNo = *status.SerialNo
		}
		logx.Infof("Tencent SMS sent successfully: Phone=%s, SerialNo=%s", phone, serialNo)
	}

	return nil
}

// GetProviderName 获取服务商名称
func (p *TencentSmsProvider) GetProviderName() string {
	return "tencent"
}

// GetTemplateCode 根据场景获取模板代码
func (p *TencentSmsProvider) GetTemplateCode(scene string) string {
	return p.getTemplateId(scene)
}

// getTemplateId 根据验证码类型获取模板 ID
func (p *TencentSmsProvider) getTemplateId(codeType string) string {
	if p.config.Templates != nil {
		if templateId, ok := p.config.Templates[codeType]; ok {
			return templateId
		}
	}

	// 默认模板 ID（如果配置中没有指定）
	// 注意：腾讯云的模板 ID 是数字字符串，需要在实际使用时配置
	defaultTemplates := map[string]string{
		"login":          "1000001",
		"register":       "1000002",
		"reset_password": "1000003",
		"bind_email":     "1000004",
		"bind_phone":     "1000005",
	}

	return defaultTemplates[codeType]
}

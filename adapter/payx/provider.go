package payx

import "fmt"

// NewPaymentProvider 创建支付服务提供商实例（工厂模式）。
// 未识别的 provider 返回错误，避免静默回落到 Mock 造成「假支付成功」。
// provider 名常量：避免字面量在多处拼错导致走错分支
const (
	ProviderAlipay = "alipay"
	ProviderWechat = "wechat"
	ProviderStripe = "stripe"
	ProviderMock   = "mock"
)

func NewPaymentProvider(config *PaymentConfig) (PaymentProvider, error) {
	switch config.Provider {
	case ProviderAlipay:
		return NewAlipayProvider(config)
	case ProviderWechat:
		return NewWechatProvider(config)
	case ProviderStripe:
		return NewStripeProvider(config), nil
	default:
		return nil, fmt.Errorf("payx: unsupported provider %q (want alipay | wechat | stripe)", config.Provider)
	}
}

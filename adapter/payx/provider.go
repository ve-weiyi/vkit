package payx

// NewPaymentProvider 创建支付服务提供商实例（工厂模式）
func NewPaymentProvider(config *PaymentConfig) PaymentProvider {
	switch config.Provider {
	case "alipay":
		return NewAlipayProvider(config)
	case "wechat":
		return NewWechatProvider(config)
	case "stripe":
		return NewStripeProvider(config)
	default:
		// 默认使用 Mock 提供商（开发环境）
		return NewMockProvider(config)
	}
}

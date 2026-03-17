package payx

// PaymentConfig 支付服务配置
type PaymentConfig struct {
	Provider string // 支付渠道：alipay | wechat | mock

	// 支付宝配置
	AppId      string // 应用ID
	PrivateKey string // 应用私钥
	PublicKey  string // 支付宝公钥
	IsProd     bool   // 是否生产环境

	// 微信配置
	MchId    string // 商户号
	SerialNo string // 证书序列号
	ApiKey   string // API密钥（V2版本 / 云购OS mchKey）
	ApiV3Key string // API V3密钥
	CertPath string // 证书路径（可选）

	// Stripe 配置
	WebhookSecret string // Stripe Webhook 签名密钥

	// 通用配置
	NotifyURL string // 默认异步回调地址
	ReturnURL string // 默认同步回调地址
}

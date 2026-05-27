package payx

import (
	"context"
	"time"
)

// CreateOrderRequest 创建订单请求
type CreateOrderRequest struct {
	OrderNo   string  // 商户订单号
	Amount    float64 // 订单金额（元）
	Subject   string  // 订单标题
	Body      string  // 订单描述
	ReturnURL string  // 同步回调地址（支付宝PC支付需要）
	NotifyURL string  // 异步回调地址
	ClientIP  string  // 客户端IP
}

// CreateOrderResponse 创建订单响应
type CreateOrderResponse struct {
	OrderNo        string // 商户订单号
	ChannelOrderNo string // 支付渠道订单号
	PayURL         string // 支付URL（支付宝PC支付）
	QRCode         string // 支付二维码（微信Native支付）
	PayData        string // 其他支付数据（JSON格式）
}

// OrderQueryResponse 订单查询响应
type OrderQueryResponse struct {
	OrderNo        string    // 商户订单号
	ChannelOrderNo string    // 支付渠道订单号
	Status         string    // 订单状态
	Amount         float64   // 订单金额（元）
	PaidAmount     float64   // 实际支付金额（元）
	PaidAt         time.Time // 支付时间
	Extra          string    // 额外信息（JSON格式）
}

// RefundRequest 退款请求
type RefundRequest struct {
	OrderNo      string  // 商户订单号
	RefundNo     string  // 退款单号
	RefundAmount float64 // 退款金额（元）
	RefundReason string  // 退款原因
	TotalAmount  float64 // 订单总金额（元）
	NotifyURL    string  // 退款异步回调地址
}

// RefundResponse 退款响应
type RefundResponse struct {
	RefundNo        string    // 退款单号
	ChannelRefundNo string    // 支付渠道退款单号
	RefundAmount    float64   // 退款金额（元）
	RefundAt        time.Time // 退款时间
	Status          string    // 退款状态
}

// NotifyData 回调通知数据
type NotifyData struct {
	OrderNo        string            // 商户订单号
	ChannelOrderNo string            // 支付渠道订单号
	Status         string            // 订单状态
	Amount         float64           // 订单金额（元）
	PaidAmount     float64           // 实际支付金额（元）
	PaidAt         time.Time         // 支付时间
	RawData        map[string]string // 原始回调数据
}

// PaymentProvider 支付服务提供商接口
// 支持多种支付渠道：支付宝、微信、Mock等
type PaymentProvider interface {
	// CreateOrder 创建支付订单
	CreateOrder(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error)

	// QueryOrder 查询订单状态
	QueryOrder(ctx context.Context, orderNo string) (*OrderQueryResponse, error)

	// CloseOrder 关闭订单
	CloseOrder(ctx context.Context, orderNo string) error

	// Refund 退款
	Refund(ctx context.Context, req *RefundRequest) (*RefundResponse, error)

	// VerifyNotifyData 验证回调签名并解析数据（从原始数据，用于 RPC 层）
	VerifyNotifyData(formData map[string]string, bodyData []byte) (*NotifyData, error)

	// GetProviderName 获取服务商名称
	GetProviderName() string
}

package payx

import (
	"context"
	"fmt"
	"time"

	"github.com/ve-weiyi/pkg/utils/randomx"
)

// MockProvider Mock支付服务提供商（用于开发测试）
type MockProvider struct {
	config *PaymentConfig
}

// NewMockProvider 创建Mock支付服务提供商
func NewMockProvider(config *PaymentConfig) *MockProvider {
	return &MockProvider{
		config: config,
	}
}

func (p *MockProvider) GetProviderName() string {
	return "mock"
}

// CreateOrder 创建Mock支付订单
func (p *MockProvider) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error) {
	return &CreateOrderResponse{
		OrderNo:        req.OrderNo,
		ChannelOrderNo: randomx.GenerateOrderNo(),
		PayURL:         fmt.Sprintf("http://mock-pay.example.com/pay?order_no=%s", req.OrderNo),
		QRCode:         fmt.Sprintf("http://mock-pay.example.com/qrcode?order_no=%s", req.OrderNo),
	}, nil
}

// QueryOrder 查询Mock订单状态
func (p *MockProvider) QueryOrder(ctx context.Context, orderNo string) (*OrderQueryResponse, error) {
	return &OrderQueryResponse{
		OrderNo:        orderNo,
		ChannelOrderNo: randomx.GenerateOrderNo(),
		Status:         "SUCCESS",
		Amount:         100.00,
		PaidAmount:     100.00,
		PaidAt:         time.Now(),
	}, nil
}

// CloseOrder 关闭Mock订单
func (p *MockProvider) CloseOrder(ctx context.Context, orderNo string) error {
	return nil
}

// Refund Mock退款
func (p *MockProvider) Refund(ctx context.Context, req *RefundRequest) (*RefundResponse, error) {
	return &RefundResponse{
		RefundNo:        req.RefundNo,
		ChannelRefundNo: randomx.GenerateOrderNo(),
		RefundAmount:    req.RefundAmount,
		RefundAt:        time.Now(),
		Status:          "SUCCESS",
	}, nil
}

// VerifyNotify 验证Mock回调签名并解析数据
func (p *MockProvider) VerifyNotify(formData map[string]string, bodyData []byte) (*NotifyData, error) {
	return p.VerifyNotifyData(formData, bodyData)
}

// VerifyNotifyData 验证Mock回调签名并解析数据（从原始数据，用于 RPC 层）
func (p *MockProvider) VerifyNotifyData(formData map[string]string, bodyData []byte) (*NotifyData, error) {
	orderNo := formData["order_no"]
	if orderNo == "" {
		return nil, fmt.Errorf("订单号不能为空")
	}

	return &NotifyData{
		OrderNo:        orderNo,
		ChannelOrderNo: randomx.GenerateOrderNo(),
		Status:         "SUCCESS",
		Amount:         100.00,
		PaidAmount:     100.00,
		PaidAt:         time.Now(),
		RawData: map[string]string{
			"order_no": orderNo,
			"status":   "SUCCESS",
		},
	}, nil
}

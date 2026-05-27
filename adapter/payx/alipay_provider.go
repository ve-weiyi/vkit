package payx

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
)

// AlipayProvider 支付宝支付服务提供商
type AlipayProvider struct {
	config *PaymentConfig
	client *alipay.Client
}

// NewAlipayProvider 创建支付宝支付服务提供商
func NewAlipayProvider(config *PaymentConfig) *AlipayProvider {
	client, err := alipay.NewClient(config.AppId, config.PrivateKey, config.IsProd)
	if err != nil {
		panic(fmt.Sprintf("初始化支付宝客户端失败: %v", err))
	}
	return &AlipayProvider{
		config: config,
		client: client,
	}
}

func (p *AlipayProvider) GetProviderName() string {
	return "alipay"
}

// CreateOrder 创建支付宝PC支付订单
func (p *AlipayProvider) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error) {
	bm := make(gopay.BodyMap)
	bm.Set("subject", req.Subject).
		Set("out_trade_no", req.OrderNo).
		Set("total_amount", fmt.Sprintf("%.2f", req.Amount)).
		Set("product_code", "FAST_INSTANT_TRADE_PAY")

	returnURL := req.ReturnURL
	if returnURL == "" {
		returnURL = p.config.ReturnURL
	}
	if returnURL != "" {
		bm.Set("return_url", returnURL)
	}

	notifyURL := req.NotifyURL
	if notifyURL == "" {
		notifyURL = p.config.NotifyURL
	}
	if notifyURL != "" {
		bm.Set("notify_url", notifyURL)
	}

	payURL, err := p.client.TradePagePay(ctx, bm)
	if err != nil {
		return nil, fmt.Errorf("创建支付宝订单失败: %w", err)
	}

	return &CreateOrderResponse{
		OrderNo: req.OrderNo,
		PayURL:  payURL,
	}, nil
}

// QueryOrder 查询支付宝订单状态
func (p *AlipayProvider) QueryOrder(ctx context.Context, orderNo string) (*OrderQueryResponse, error) {
	bm := make(gopay.BodyMap)
	bm.Set("out_trade_no", orderNo)

	rsp, err := p.client.TradeQuery(ctx, bm)
	if err != nil {
		return nil, fmt.Errorf("查询支付宝订单失败: %w", err)
	}

	if rsp.Response == nil {
		return nil, fmt.Errorf("查询支付宝订单返回为空")
	}

	result := &OrderQueryResponse{
		OrderNo:        orderNo,
		ChannelOrderNo: rsp.Response.TradeNo,
		Status:         rsp.Response.TradeStatus,
	}

	if rsp.Response.TotalAmount != "" {
		amount, _ := strconv.ParseFloat(rsp.Response.TotalAmount, 64)
		result.Amount = amount
	}
	if rsp.Response.BuyerPayAmount != "" {
		paidAmount, _ := strconv.ParseFloat(rsp.Response.BuyerPayAmount, 64)
		result.PaidAmount = paidAmount
	}
	if rsp.Response.SendPayDate != "" {
		paidAt, err := time.Parse("2006-01-02 15:04:05", rsp.Response.SendPayDate)
		if err == nil {
			result.PaidAt = paidAt
		}
	}

	return result, nil
}

// CloseOrder 关闭支付宝订单
func (p *AlipayProvider) CloseOrder(ctx context.Context, orderNo string) error {
	bm := make(gopay.BodyMap)
	bm.Set("out_trade_no", orderNo)

	_, err := p.client.TradeClose(ctx, bm)
	if err != nil {
		return fmt.Errorf("关闭支付宝订单失败: %w", err)
	}

	return nil
}

// Refund 支付宝退款
func (p *AlipayProvider) Refund(ctx context.Context, req *RefundRequest) (*RefundResponse, error) {
	bm := make(gopay.BodyMap)
	bm.Set("out_trade_no", req.OrderNo).
		Set("refund_amount", fmt.Sprintf("%.2f", req.RefundAmount)).
		Set("out_request_no", req.RefundNo)

	if req.RefundReason != "" {
		bm.Set("refund_reason", req.RefundReason)
	}

	rsp, err := p.client.TradeRefund(ctx, bm)
	if err != nil {
		return nil, fmt.Errorf("支付宝退款失败: %w", err)
	}

	if rsp.Response == nil {
		return nil, fmt.Errorf("支付宝退款返回为空")
	}

	result := &RefundResponse{
		RefundNo: req.RefundNo,
		Status:   "SUCCESS",
		RefundAt: time.Now(),
	}

	if rsp.Response.RefundFee != "" {
		refundAmount, _ := strconv.ParseFloat(rsp.Response.RefundFee, 64)
		result.RefundAmount = refundAmount
	}

	return result, nil
}

// VerifyNotifyData 验证支付宝回调签名并解析数据（从原始数据，用于 RPC 层）
func (p *AlipayProvider) VerifyNotifyData(formData map[string]string, bodyData []byte) (*NotifyData, error) {
	// 验证签名
	if p.config.PublicKey == "" {
		return nil, fmt.Errorf("支付宝公钥未配置")
	}

	ok, err := alipay.VerifySign(p.config.PublicKey, formData)
	if err != nil {
		return nil, fmt.Errorf("支付宝验签失败: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("支付宝签名验证不通过")
	}

	// 解析回调数据
	outTradeNo := formData["out_trade_no"]
	tradeNo := formData["trade_no"]
	tradeStatus := formData["trade_status"]
	totalAmount := formData["total_amount"]

	result := &NotifyData{
		OrderNo:        outTradeNo,
		ChannelOrderNo: tradeNo,
		Status:         tradeStatus,
		RawData:        formData,
	}

	if totalAmount != "" {
		amount, _ := strconv.ParseFloat(totalAmount, 64)
		result.Amount = amount
		result.PaidAmount = amount
	}

	if gmtPayment := formData["gmt_payment"]; gmtPayment != "" {
		paidAt, err := time.Parse("2006-01-02 15:04:05", gmtPayment)
		if err == nil {
			result.PaidAt = paidAt
		}
	}

	return result, nil
}

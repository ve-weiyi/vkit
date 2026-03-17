package payx

import (
	"context"
	"encoding/xml"
	"fmt"
	"time"

	"github.com/go-pay/gopay"
	wechatv2 "github.com/go-pay/gopay/wechat"
	wechatv3 "github.com/go-pay/gopay/wechat/v3"
)

// WechatProvider 微信支付服务提供商
type WechatProvider struct {
	config *PaymentConfig
	client *wechatv3.ClientV3
}

// NewWechatProvider 创建微信支付服务提供商
func NewWechatProvider(config *PaymentConfig) *WechatProvider {
	client, err := wechatv3.NewClientV3(config.MchId, config.SerialNo, config.ApiV3Key, config.PrivateKey)
	if err != nil {
		panic(fmt.Sprintf("初始化微信客户端失败: %v", err))
	}
	return &WechatProvider{
		config: config,
		client: client,
	}
}

func (p *WechatProvider) GetProviderName() string {
	return "wechat"
}

// CreateOrder 创建微信Native支付订单
func (p *WechatProvider) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error) {
	bm := make(gopay.BodyMap)
	bm.Set("description", req.Subject).
		Set("out_trade_no", req.OrderNo).
		Set("amount", map[string]interface{}{
			"total":    int(req.Amount * 100), // 转换为分
			"currency": "CNY",
		})

	notifyURL := req.NotifyURL
	if notifyURL == "" {
		notifyURL = p.config.NotifyURL
	}
	if notifyURL != "" {
		bm.Set("notify_url", notifyURL)
	}

	rsp, err := p.client.V3TransactionNative(ctx, bm)
	if err != nil {
		return nil, fmt.Errorf("创建微信订单失败: %w", err)
	}

	if rsp.Response == nil {
		return nil, fmt.Errorf("创建微信订单返回为空")
	}

	return &CreateOrderResponse{
		OrderNo: req.OrderNo,
		QRCode:  rsp.Response.CodeUrl,
	}, nil
}

// QueryOrder 查询微信订单状态
func (p *WechatProvider) QueryOrder(ctx context.Context, orderNo string) (*OrderQueryResponse, error) {
	rsp, err := p.client.V3TransactionQueryOrder(ctx, wechatv3.OutTradeNo, orderNo)
	if err != nil {
		return nil, fmt.Errorf("查询微信订单失败: %w", err)
	}

	if rsp.Response == nil {
		return nil, fmt.Errorf("查询微信订单返回为空")
	}

	result := &OrderQueryResponse{
		OrderNo:        orderNo,
		ChannelOrderNo: rsp.Response.TransactionId,
		Status:         rsp.Response.TradeState,
	}

	if rsp.Response.Amount != nil {
		result.Amount = float64(rsp.Response.Amount.Total) / 100
		result.PaidAmount = float64(rsp.Response.Amount.PayerTotal) / 100
	}

	if rsp.Response.SuccessTime != "" {
		paidAt, err := time.Parse(time.RFC3339, rsp.Response.SuccessTime)
		if err == nil {
			result.PaidAt = paidAt
		}
	}

	return result, nil
}

// CloseOrder 关闭微信订单
func (p *WechatProvider) CloseOrder(ctx context.Context, orderNo string) error {
	_, err := p.client.V3TransactionCloseOrder(ctx, orderNo)
	if err != nil {
		return fmt.Errorf("关闭微信订单失败: %w", err)
	}

	return nil
}

// Refund 微信退款
func (p *WechatProvider) Refund(ctx context.Context, req *RefundRequest) (*RefundResponse, error) {
	bm := make(gopay.BodyMap)
	bm.Set("out_trade_no", req.OrderNo).
		Set("out_refund_no", req.RefundNo).
		Set("amount", map[string]interface{}{
			"refund":   int(req.RefundAmount * 100), // 转换为分
			"total":    int(req.TotalAmount * 100),
			"currency": "CNY",
		})

	if req.RefundReason != "" {
		bm.Set("reason", req.RefundReason)
	}

	if req.NotifyURL != "" {
		bm.Set("notify_url", req.NotifyURL)
	}

	rsp, err := p.client.V3Refund(ctx, bm)
	if err != nil {
		return nil, fmt.Errorf("微信退款失败: %w", err)
	}

	if rsp.Response == nil {
		return nil, fmt.Errorf("微信退款返回为空")
	}

	result := &RefundResponse{
		RefundNo:        req.RefundNo,
		ChannelRefundNo: rsp.Response.RefundId,
		Status:          rsp.Response.Status,
	}

	if rsp.Response.Amount != nil {
		result.RefundAmount = float64(rsp.Response.Amount.Refund) / 100
	}

	if rsp.Response.SuccessTime != "" {
		refundAt, err := time.Parse(time.RFC3339, rsp.Response.SuccessTime)
		if err == nil {
			result.RefundAt = refundAt
		}
	}

	return result, nil
}

// WechatNotifyRequest 微信支付回调请求结构（V2版本）
type WechatNotifyRequest struct {
	XMLName       xml.Name `xml:"xml"`
	ReturnCode    string   `xml:"return_code"`
	ReturnMsg     string   `xml:"return_msg"`
	AppId         string   `xml:"appid"`
	MchId         string   `xml:"mch_id"`
	NonceStr      string   `xml:"nonce_str"`
	Sign          string   `xml:"sign"`
	ResultCode    string   `xml:"result_code"`
	OutTradeNo    string   `xml:"out_trade_no"`
	TransactionId string   `xml:"transaction_id"`
	TotalFee      int      `xml:"total_fee"`
	TimeEnd       string   `xml:"time_end"`
}

// VerifyNotifyData 验证微信回调签名并解析数据（从原始数据，用于 RPC 层）
func (p *WechatProvider) VerifyNotifyData(formData map[string]string, bodyData []byte) (*NotifyData, error) {
	var req WechatNotifyRequest
	if err := xml.Unmarshal(bodyData, &req); err != nil {
		return nil, fmt.Errorf("解析微信回调XML失败: %w", err)
	}

	// 验证返回状态
	if req.ReturnCode != "SUCCESS" {
		return nil, fmt.Errorf("微信回调返回失败: %s", req.ReturnMsg)
	}

	// 验证业务结果
	if req.ResultCode != "SUCCESS" {
		return nil, fmt.Errorf("微信支付业务失败")
	}

	// 验证签名
	if p.config.ApiKey == "" {
		return nil, fmt.Errorf("微信支付ApiKey未配置")
	}

	notifyData := make(gopay.BodyMap)
	notifyData.Set("return_code", req.ReturnCode).
		Set("return_msg", req.ReturnMsg).
		Set("appid", req.AppId).
		Set("mch_id", req.MchId).
		Set("nonce_str", req.NonceStr).
		Set("result_code", req.ResultCode).
		Set("out_trade_no", req.OutTradeNo).
		Set("transaction_id", req.TransactionId).
		Set("total_fee", req.TotalFee).
		Set("time_end", req.TimeEnd)

	ok, err := wechatv2.VerifySign(p.config.ApiKey, wechatv2.SignType_MD5, notifyData)
	if err != nil {
		return nil, fmt.Errorf("微信验签失败: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("微信签名验证不通过")
	}

	// 解析回调数据
	result := &NotifyData{
		OrderNo:        req.OutTradeNo,
		ChannelOrderNo: req.TransactionId,
		Status:         "SUCCESS",
		Amount:         float64(req.TotalFee) / 100,
		PaidAmount:     float64(req.TotalFee) / 100,
		RawData: map[string]string{
			"return_code":    req.ReturnCode,
			"result_code":    req.ResultCode,
			"out_trade_no":   req.OutTradeNo,
			"transaction_id": req.TransactionId,
			"time_end":       req.TimeEnd,
		},
	}

	if req.TimeEnd != "" {
		paidAt, err := time.Parse("20060102150405", req.TimeEnd)
		if err == nil {
			result.PaidAt = paidAt
		}
	}

	return result, nil
}

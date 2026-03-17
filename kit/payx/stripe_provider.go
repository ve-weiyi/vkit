package payx

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/webhook"
)

// StripeProvider Stripe支付服务提供商
type StripeProvider struct {
	config *PaymentConfig
}

// NewStripeProvider 创建Stripe支付服务提供商
func NewStripeProvider(config *PaymentConfig) *StripeProvider {
	stripe.Key = config.PrivateKey
	return &StripeProvider{config: config}
}

func (p *StripeProvider) GetProviderName() string {
	return "stripe"
}

// CreateOrder 创建Stripe Checkout Session
func (p *StripeProvider) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error) {
	returnURL := req.ReturnURL
	if returnURL == "" {
		returnURL = p.config.ReturnURL
	}
	if returnURL == "" {
		return nil, fmt.Errorf("stripe支付需要配置return_url")
	}

	amountCents := int64(math.Round(req.Amount * 100))

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("usd"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(req.Subject),
					},
					UnitAmount: stripe.Int64(amountCents),
				},
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: stripe.String(returnURL + "?session_id={CHECKOUT_SESSION_ID}&order_no=" + req.OrderNo + "&status=success"),
		CancelURL:  stripe.String(returnURL + "?order_no=" + req.OrderNo + "&status=cancel"),
		Metadata: map[string]string{
			"order_no": req.OrderNo,
		},
	}

	s, err := session.New(params)
	if err != nil {
		return nil, fmt.Errorf("创建Stripe Checkout Session失败: %w", err)
	}

	return &CreateOrderResponse{
		OrderNo:        req.OrderNo,
		ChannelOrderNo: s.ID,
		PayURL:         s.URL,
	}, nil
}

// QueryOrder 查询Stripe Checkout Session状态
func (p *StripeProvider) QueryOrder(ctx context.Context, channelOrderNo string) (*OrderQueryResponse, error) {
	s, err := session.Get(channelOrderNo, nil)
	if err != nil {
		return nil, fmt.Errorf("查询Stripe Session失败: %w", err)
	}

	status := "PENDING"
	if s.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid {
		status = "SUCCESS"
	}

	orderNo := ""
	if s.Metadata != nil {
		orderNo = s.Metadata["order_no"]
	}

	return &OrderQueryResponse{
		OrderNo:        orderNo,
		ChannelOrderNo: s.ID,
		Status:         status,
		Amount:         float64(s.AmountTotal) / 100,
		PaidAmount:     float64(s.AmountSubtotal) / 100,
	}, nil
}

// CloseOrder 关闭Stripe Checkout Session
func (p *StripeProvider) CloseOrder(ctx context.Context, channelOrderNo string) error {
	_, err := session.Expire(channelOrderNo, nil)
	if err != nil {
		return fmt.Errorf("关闭Stripe Session失败: %w", err)
	}
	return nil
}

// Refund Stripe退款
func (p *StripeProvider) Refund(ctx context.Context, req *RefundRequest) (*RefundResponse, error) {
	return nil, fmt.Errorf("stripe退款暂未实现")
}

// VerifyNotifyData 验证Stripe webhook签名并解析数据
func (p *StripeProvider) VerifyNotifyData(formData map[string]string, bodyData []byte) (*NotifyData, error) {
	webhookSecret := p.config.WebhookSecret
	sigHeader := formData["stripe-signature"]

	if webhookSecret == "" {
		return nil, fmt.Errorf("stripe webhook secret未配置")
	}
	if sigHeader == "" {
		return nil, fmt.Errorf("缺少stripe-signature header")
	}

	event, err := webhook.ConstructEvent(bodyData, sigHeader, webhookSecret)
	if err != nil {
		return nil, fmt.Errorf("stripe webhook验签失败: %w", err)
	}

	switch event.Type {
	case "checkout.session.completed", "checkout.session.async_payment_failed":
		// 继续处理
	default:
		return nil, fmt.Errorf("不处理的事件类型: %s", event.Type)
	}

	var s stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &s); err != nil {
		return nil, fmt.Errorf("解析stripe session失败: %w", err)
	}

	orderNo := ""
	if s.Metadata != nil {
		orderNo = s.Metadata["order_no"]
	}

	status := "FAILED"
	if s.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid {
		status = "SUCCESS"
	}

	paidAt := time.Unix(event.Created, 0)

	return &NotifyData{
		OrderNo:        orderNo,
		ChannelOrderNo: s.ID,
		Status:         status,
		Amount:         float64(s.AmountTotal) / 100,
		PaidAmount:     float64(s.AmountTotal) / 100,
		PaidAt:         paidAt,
		RawData:        formData,
	}, nil
}

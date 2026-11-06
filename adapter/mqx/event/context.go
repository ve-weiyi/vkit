package event

import (
	"context"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

type deliveryKey struct{}

func withDelivery(ctx context.Context, d *mqx.Delivery) context.Context {
	return context.WithValue(ctx, deliveryKey{}, d)
}

// DeliveryFrom 返回事件处理上下文中的投递句柄。
//
// 业务需要事件头（event_id、schema_version 等）或手动确认时使用。
func DeliveryFrom(ctx context.Context) (*mqx.Delivery, bool) {
	d, ok := ctx.Value(deliveryKey{}).(*mqx.Delivery)
	return d, ok
}

// EventIDFrom 返回事件的稳定唯一标识；消费侧用它做幂等键。
func EventIDFrom(ctx context.Context) string {
	if d, ok := DeliveryFrom(ctx); ok {
		return d.ID
	}
	return ""
}

// HeaderFrom 返回事件头；不存在时返回空串。
func HeaderFrom(ctx context.Context, key string) string {
	if d, ok := DeliveryFrom(ctx); ok {
		return d.Headers[key]
	}
	return ""
}

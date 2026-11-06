// Package mqx 定义消息队列的最小 SPI。
//
// 核心只承诺四个后端都能诚实兑现的能力：发布、订阅、确认、关闭。
// 后端差异由各实现的 Config 承载；可选能力用独立接口声明。
package mqx

import "context"

// Broker 管理连接与订阅生命周期。
type Broker interface {
	Publisher() (Publisher, error)
	Subscriber() (Subscriber, error)
	Ping(ctx context.Context) error
	// Close 幂等：先停止全部订阅，再关闭连接。
	Close(ctx context.Context) error
}

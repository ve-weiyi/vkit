package mqx

import (
	"sync"
	"sync/atomic"
)

// ManualAck 管理「handler 手动确认」的状态。
//
// AutoAck=false 时各后端用 Inject 把只生效一次的 Ack/Nack 注入到 Message；
// 处理流程在 error 路径读取 Manual() 以跳过库的默认确认/DLQ，
// 避免覆盖 handler 自己的确认决策。零值即可使用。
type ManualAck struct {
	manual atomic.Bool
	once   sync.Once
}

// Inject 把手动确认函数注入 msg：Ack 与 Nack 由同一个 sync.Once 互斥，
// 只有先到的那一个生效；任一个被调用都会置位手动标记。
func (m *ManualAck) Inject(msg *Message, ack func() error, nack func(requeue bool) error) {
	msg.AckFn = func() error {
		m.manual.Store(true)
		var err error
		m.once.Do(func() { err = ack() })
		return err
	}
	msg.NackFn = func(requeue bool) error {
		m.manual.Store(true)
		var err error
		m.once.Do(func() { err = nack(requeue) })
		return err
	}
}

// Manual 返回 handler 是否已通过 AckFn/NackFn 手动确认过。
func (m *ManualAck) Manual() bool { return m.manual.Load() }

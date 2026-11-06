package mqx

import "sync"

// Delivery 是投递句柄：消息数据 + 确认能力 + 投递元数据。
//
// 适配器负责构造 Delivery 并注入 ack/nack；handler 可通过 Ack/Nack 接管确认，
// 一旦接管，库不再按返回值做默认处理。
type Delivery struct {
	Message

	Partition   int32
	Offset      int64
	Redelivered bool
	// Attempt 是当前进程内的第几次处理，从 1 开始，由 Process 维护。
	Attempt int

	mu      sync.Mutex
	settled bool
	ack     func() error
	nack    func(requeue bool) error
}

// NewDelivery 由适配器构造投递句柄。ack/nack 可为 nil（表示该后端无需确认）。
func NewDelivery(msg Message, ack func() error, nack func(requeue bool) error) *Delivery {
	return &Delivery{Message: msg, ack: ack, nack: nack}
}

func (d *Delivery) take() (ack func() error, nack func(requeue bool) error, ok bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.settled {
		return nil, nil, false
	}
	d.settled = true
	return d.ack, d.nack, true
}

// Ack 确认消息处理成功。多次调用只有首次生效，且与 Nack 互斥。
func (d *Delivery) Ack() error {
	ack, _, ok := d.take()
	if !ok || ack == nil {
		return nil
	}
	return ack()
}

// Nack 拒绝消息。requeue=true 表示尽力重投（各后端语义见实现矩阵）。
// 多次调用只有首次生效，且与 Ack 互斥。
func (d *Delivery) Nack(requeue bool) error {
	_, nack, ok := d.take()
	if !ok || nack == nil {
		return nil
	}
	return nack(requeue)
}

// Settled 返回 handler 是否已手动 Ack 或 Nack。
func (d *Delivery) Settled() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.settled
}

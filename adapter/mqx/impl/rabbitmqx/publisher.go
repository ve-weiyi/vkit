package rabbitmqx

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// publisher 维护一组开启 confirm 的 channel，发布时轮询选择，避免单 channel 串行化。
type publisher struct {
	broker   *Broker
	channels []*pubChannel
	next     atomic.Uint64
}

type pubChannel struct {
	mu         sync.Mutex
	ch         *amqp.Channel
	returns    chan amqp.Return
	closeCh    chan *amqp.Error
	generation uint64
	declared   map[string]struct{}
}

var (
	_ mqx.Publisher      = (*publisher)(nil)
	_ mqx.BatchPublisher = (*publisher)(nil)
)

func newPublisher(broker *Broker) *publisher {
	p := &publisher{broker: broker, channels: make([]*pubChannel, broker.cfg.PublisherPoolSize)}
	for i := range p.channels {
		p.channels[i] = &pubChannel{}
	}
	return p
}

// Publish 实现 mqx.Publisher。
func (p *publisher) Publish(ctx context.Context, msg *mqx.Message) (*mqx.PublishResult, error) {
	if msg == nil {
		return nil, fmt.Errorf("%w: message is nil", mqx.ErrBadMessage)
	}
	prepared, err := msg.Prepared()
	if err != nil {
		return nil, err
	}

	start := time.Now()
	err = p.publish(ctx, &prepared)
	if metrics := p.broker.cfg.Metrics; metrics != nil {
		metrics.PublishCompleted(prepared.Destination, prepared.RoutingKey, time.Since(start), err)
	}
	if err != nil {
		return nil, err
	}
	return &mqx.PublishResult{ID: prepared.ID}, nil
}

func (p *publisher) publish(ctx context.Context, msg *mqx.Message) error {
	pc := p.pick()
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if err := pc.ensure(p.broker, msg.Destination); err != nil {
		return err
	}
	// 清掉上一轮可能残留的 return，避免把旧消息的不可路由结果算到本条上。
	pc.drainReturns()

	confirm, err := pc.ch.PublishWithDeferredConfirmWithContext(
		ctx, msg.Destination, msg.RoutingKey, true, false, toAMQP(msg),
	)
	if err != nil {
		cause := pc.closeCause(err)
		pc.reset()
		return ClassifyPublishError(cause)
	}
	if confirm == nil {
		return fmt.Errorf("%w: publisher confirm unavailable", mqx.ErrUnavailable)
	}

	ack, err := confirm.WaitContext(ctx)
	if err != nil {
		cause := pc.closeCause(err)
		pc.reset()
		return ClassifyPublishError(cause)
	}

	// mandatory=true 时，exchange 没有匹配队列会触发 basic.return，但 confirm 仍可能 ack。
	// 必须在确认后检查 return，否则会把「没人接收」当成「发布成功」。
	if returned, ok := pc.takeReturn(); ok {
		return fmt.Errorf("%w: message unroutable (exchange=%s routing_key=%s): %s",
			mqx.ErrUnavailable, returned.Exchange, returned.RoutingKey, returned.ReplyText)
	}
	if !ack {
		return fmt.Errorf("%w: message nacked by broker", mqx.ErrUnavailable)
	}
	return nil
}

func (pc *pubChannel) drainReturns() {
	if pc.returns == nil {
		return
	}
	for {
		select {
		case _, ok := <-pc.returns:
			if !ok {
				// channel 关闭时 amqp 会关闭 NotifyReturn；继续读会空转。
				pc.returns = nil
				return
			}
		default:
			return
		}
	}
}

func (pc *pubChannel) takeReturn() (amqp.Return, bool) {
	if pc.returns == nil {
		return amqp.Return{}, false
	}
	select {
	case returned, ok := <-pc.returns:
		if !ok {
			pc.returns = nil
			return amqp.Return{}, false
		}
		return returned, true
	default:
		return amqp.Return{}, false
	}
}

// PublishBatch 先全部投递，再统一等待 confirm。
// 批量路径不检测 unroutable；需要该保证时逐条调用 Publish。
func (p *publisher) PublishBatch(ctx context.Context, msgs []*mqx.Message) ([]mqx.PublishResult, error) {
	prepared := make([]mqx.Message, len(msgs))
	for i, msg := range msgs {
		if msg == nil {
			return nil, fmt.Errorf("%w: message %d is nil", mqx.ErrBadMessage, i)
		}
		item, err := msg.Prepared()
		if err != nil {
			return nil, err
		}
		prepared[i] = item
	}

	results := make([]mqx.PublishResult, len(prepared))
	pc := p.pick()
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.drainReturns()

	for _, msg := range prepared {
		if err := pc.ensure(p.broker, msg.Destination); err != nil {
			return results, err
		}
	}

	confirms := make([]*amqp.DeferredConfirmation, len(prepared))
	for i := range prepared {
		confirm, err := pc.ch.PublishWithDeferredConfirmWithContext(
			ctx, prepared[i].Destination, prepared[i].RoutingKey, false, false, toAMQP(&prepared[i]),
		)
		if err != nil {
			cause := pc.closeCause(err)
			pc.reset()
			return results, ClassifyPublishError(cause)
		}
		confirms[i] = confirm
	}

	for i, confirm := range confirms {
		if confirm == nil {
			results[i] = mqx.PublishResult{ID: prepared[i].ID}
			continue
		}
		ack, err := confirm.WaitContext(ctx)
		if err != nil {
			cause := pc.closeCause(err)
			pc.reset()
			return results, ClassifyPublishError(cause)
		}
		if !ack {
			return results, fmt.Errorf("%w: message %d nacked by broker", mqx.ErrUnavailable, i)
		}
		results[i] = mqx.PublishResult{ID: prepared[i].ID}
	}
	return results, nil
}

func (p *publisher) pick() *pubChannel {
	index := int(p.next.Add(1)) % len(p.channels)
	return p.channels[index]
}

// ensure 在连接代际变化或 channel 未就绪时重建 channel，并声明目标 exchange。
func (pc *pubChannel) ensure(broker *Broker, exchange string) error {
	generation := broker.conn.Generation()
	if pc.ch == nil || pc.generation != generation {
		ch, err := broker.conn.Channel()
		if err != nil {
			return err
		}
		if err := ch.Confirm(false); err != nil {
			_ = ch.Close()
			return fmt.Errorf("%w: enable publisher confirm: %v", mqx.ErrUnavailable, err)
		}
		if pc.ch != nil {
			_ = pc.ch.Close()
		}
		pc.ch = ch
		pc.returns = ch.NotifyReturn(make(chan amqp.Return, 4))
		pc.closeCh = ch.NotifyClose(make(chan *amqp.Error, 1))
		pc.generation = generation
		pc.declared = make(map[string]struct{})
	}

	if _, ok := pc.declared[exchange]; ok {
		return nil
	}
	if err := broker.topology.declareExchange(pc.ch, exchange); err != nil {
		pc.reset()
		return ClassifyPublishError(err)
	}
	pc.declared[exchange] = struct{}{}
	return nil
}

func (pc *pubChannel) reset() {
	if pc.ch != nil {
		_ = pc.ch.Close()
	}
	pc.ch = nil
	pc.returns = nil
	pc.closeCh = nil
	pc.generation = 0
	pc.declared = nil
}

// closeCause 优先返回 channel 关闭时 broker 给出的协议错误（如 403/406），
// 让调用方能区分永久配置错误与瞬时连接问题。
func (pc *pubChannel) closeCause(err error) error {
	if pc.closeCh == nil {
		return err
	}
	if !errors.Is(err, amqp.ErrClosed) {
		return err
	}

	// amqp 在 channel 关闭时先投递 *amqp.Error，再把 ErrClosed 返回给调用方；
	// 这里短暂等待关闭原因，避免把 403/406 这类永久错误误判为瞬时错误。
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case closeErr := <-pc.closeCh:
		if closeErr != nil {
			return closeErr
		}
	case <-timer.C:
	}
	return err
}

// ClassifyPublishError 把底层发布错误转换为 mqx 错误：
// 401/403/404/405/406 等永久性 broker 错误用 mqx.Permanent 包装，其余返回 ErrUnavailable。
func ClassifyPublishError(err error) error {
	var amqpErr *amqp.Error
	if errors.As(err, &amqpErr) {
		switch amqpErr.Code {
		case 401, 403, 404, 405, 406:
			return mqx.Permanent(fmt.Errorf("%w: %s (%d)", mqx.ErrUnavailable, amqpErr.Reason, amqpErr.Code))
		}
	}
	return fmt.Errorf("%w: %v", mqx.ErrUnavailable, err)
}

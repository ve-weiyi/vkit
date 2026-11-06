package mqx

import "context"

// Publisher 发布单条消息。
type Publisher interface {
	Publish(ctx context.Context, msg *Message) (*PublishResult, error)
}

// BatchPublisher 是可选能力；不支持的后端返回 ErrNotSupported。
type BatchPublisher interface {
	PublishBatch(ctx context.Context, msgs []*Message) ([]PublishResult, error)
}

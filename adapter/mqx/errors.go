package mqx

import "errors"

var (
	// ErrTimeout 操作超时。
	ErrTimeout = errors.New("mqx: operation timeout")

	// ErrNotSupported 当前后端不支持该能力。
	ErrNotSupported = errors.New("mqx: capability not supported by this backend")

	// ErrClosed Publisher/Subscriber/MessageQueue 已关闭。
	ErrClosed = errors.New("mqx: closed")

	// ErrNotConnected 与 broker 未连接。
	ErrNotConnected = errors.New("mqx: not connected to broker")

	// ErrSendFailed 消息发送失败。
	ErrSendFailed = errors.New("mqx: send failed")

	// ErrBadMessage 无效的消息。
	ErrBadMessage = errors.New("mqx: invalid message")

	// ErrSubscribeFailed 订阅失败。
	ErrSubscribeFailed = errors.New("mqx: subscribe failed")

	// ErrHandlerTimeout handler 处理超时。
	ErrHandlerTimeout = errors.New("mqx: handler timeout")
)

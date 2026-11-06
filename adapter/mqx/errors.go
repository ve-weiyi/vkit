package mqx

import (
	"errors"
	"time"
)

var (
	// ErrUnavailable broker 未就绪或不可达，消息未被接受。
	ErrUnavailable = errors.New("mqx: broker unavailable")
	// ErrClosed 资源已关闭。
	ErrClosed = errors.New("mqx: closed")
	// ErrNotSupported 当前后端不支持该能力。
	ErrNotSupported = errors.New("mqx: capability not supported by this backend")
	// ErrBadMessage 消息不合法。
	ErrBadMessage = errors.New("mqx: invalid message")
	// ErrInvalidSpec 订阅规格不合法。
	ErrInvalidSpec = errors.New("mqx: invalid subscription spec")
)

// permanentError 表示不可重试的错误。
type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

// Permanent 标记错误不可重试：handler 返回它时直接进入死信或丢弃，不再重试。
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	if IsPermanent(err) {
		return err
	}
	return &permanentError{err: err}
}

// IsPermanent 判断错误是否被标记为不可重试。
func IsPermanent(err error) bool {
	var target *permanentError
	return errors.As(err, &target)
}

// retryAfterError 表示按指定延迟重试。
type retryAfterError struct {
	err   error
	delay time.Duration
}

func (e *retryAfterError) Error() string { return e.err.Error() }
func (e *retryAfterError) Unwrap() error { return e.err }

// RetryAfter 标记错误在指定延迟后重试；延迟仍受 RetryPolicy.MaxDelay 约束。
func RetryAfter(err error, delay time.Duration) error {
	if err == nil {
		return nil
	}
	return &retryAfterError{err: err, delay: delay}
}

// RetryDelay 读取错误携带的重试延迟。
func RetryDelay(err error) (time.Duration, bool) {
	var target *retryAfterError
	if errors.As(err, &target) {
		return target.delay, true
	}
	return 0, false
}

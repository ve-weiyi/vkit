package codestore

import (
	"time"
)

// 验证码存储管理核心接口
type CodeStore interface {
	// 生成并存储验证码（自动覆盖旧验证码）
	Generate(key string, length int, expire time.Duration) (string, error)

	// 验证验证码
	Verify(key string, code string) (bool, error)
}

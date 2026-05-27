package codestore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ve-weiyi/vkit/adapter/storex"

	"github.com/ve-weiyi/vkit/x/randomx"
)

// CodeStore 基于 KVStore 的 CodeStore 实现
type CodeStore struct {
	store storex.KVStore
}

// NewCodeStore 创建 CodeStore 实例
func NewCodeStore(store storex.KVStore) *CodeStore {
	return &CodeStore{
		store: store,
	}
}

// Generate 生成并存储验证码（自动覆盖旧验证码）
func (r *CodeStore) Generate(key string, length int, expire time.Duration) (string, error) {
	if key == "" {
		return "", errors.New("key is empty")
	}
	if length <= 0 {
		return "", errors.New("code length must > 0")
	}
	if expire <= 0 {
		expire = 5 * time.Minute
	}

	code := randomx.GenerateCode(length)
	ctx := context.Background()
	err := r.store.Set(ctx, key, code, expire)
	if err != nil {
		return "", fmt.Errorf("store code failed: %v", err)
	}

	return code, nil
}

// Verify 验证验证码（自动判断过期）
func (r *CodeStore) Verify(key string, code string) (bool, error) {
	if key == "" || code == "" {
		return false, errors.New("key or code is empty")
	}

	ctx := context.Background()
	storedCode, err := r.store.Get(ctx, key)
	if err != nil {
		return false, fmt.Errorf("get code failed: %v", err)
	}
	if storedCode == "" {
		return false, nil
	}
	if storedCode != code {
		return false, nil
	}

	_ = r.store.Delete(ctx, key)
	return true, nil
}

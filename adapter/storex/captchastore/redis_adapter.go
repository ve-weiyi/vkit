package captchastore

import (
	"context"
	"time"

	"github.com/ve-weiyi/vkit/adapter/storex"
)

// 默认过期时间
const defaultExpiration = 15 * time.Minute

// storeAdapter 内部适配器，将 KVStore 适配为 base64Captcha.Store
type storeAdapter struct {
	store      storex.KVStore
	expiration time.Duration // 过期时间属性
}

// newStoreAdapter 创建适配器实例
func newStoreAdapter(store storex.KVStore, expiration time.Duration) *storeAdapter {
	if expiration <= 0 {
		expiration = defaultExpiration
	}
	return &storeAdapter{
		store:      store,
		expiration: expiration,
	}
}

func (a *storeAdapter) Set(key string, value string) error {
	return a.store.Set(context.Background(), key, value, a.expiration)
}

func (a *storeAdapter) Get(key string, clear bool) string {
	ctx := context.Background()
	val, err := a.store.Get(ctx, key)
	if err != nil || val == "" {
		return ""
	}
	if clear {
		_ = a.store.Delete(ctx, key)
	}
	return val
}

func (a *storeAdapter) Verify(key, answer string, clear bool) bool {
	v := a.Get(key, clear)
	return v != "" && v == answer
}

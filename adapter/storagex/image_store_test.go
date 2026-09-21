package storagex

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newLocalImageStore 构造基于临时目录 local provider 的 ImageStore。
func newLocalImageStore(t *testing.T) (ImageStore, string) {
	t.Helper()
	dir := t.TempDir()
	provider := NewLocalStorageProvider(&LocalConfig{Dir: dir})
	return NewImageStore(provider), dir
}

func TestImageStoreSaveGet(t *testing.T) {
	store, _ := newLocalImageStore(t)
	ctx := context.Background()

	key := "B-1/s1/0_0_1.png"
	if err := store.Save(ctx, key, []byte("img-data")); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	got, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if string(got) != "img-data" {
		t.Fatalf("Get body = %q, want img-data", got)
	}
}

func TestImageStoreGetNotFound(t *testing.T) {
	store, _ := newLocalImageStore(t)
	_, err := store.Get(context.Background(), "missing/0.png")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get error = %v, want ErrNotFound", err)
	}
}

func TestImageStoreURL(t *testing.T) {
	store, _ := newLocalImageStore(t)
	// local provider 无 Endpoint/CDN 时默认相对路径 /static/<key>（与网关路由对齐）
	if got := store.URL("B-1/s1/0.png"); got != "/static/B-1/s1/0.png" {
		t.Fatalf("URL = %q, want /static/B-1/s1/0.png", got)
	}
}

func TestImageStoreSavePersistsToDisk(t *testing.T) {
	store, dir := newLocalImageStore(t)
	key := "a/b/c.png"
	if err := store.Save(context.Background(), key, []byte("data")); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	full := filepath.Join(dir, key)
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("read persisted file: %v", err)
	}
	if string(data) != "data" {
		t.Fatalf("persisted body = %q, want data", data)
	}
}

// TestLocalRetentionAndCleanExpired 验证 local provider 保留期：WithRetentionDays 启用保留期、
// cleanExpired 按 modtime 扫目录删过期文件（未过期保留）、Close 停协程。
func TestLocalRetentionAndCleanExpired(t *testing.T) {
	dir := t.TempDir()
	p := NewLocalStorageProvider(&LocalConfig{Dir: dir}, WithRetentionDays(1))
	defer p.Close()
	store := NewImageStore(p)
	ctx := context.Background()

	// 过期文件：modtime 置为 2 天前 → 应被清理
	keyOld := "a/old.png"
	if err := store.Save(ctx, keyOld, []byte("data")); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	old := time.Now().AddDate(0, 0, -2)
	if err := os.Chtimes(filepath.Join(dir, keyOld), old, old); err != nil {
		t.Fatalf("Chtimes error: %v", err)
	}

	// 未过期文件：应保留
	keyFresh := "a/fresh.png"
	if err := store.Save(ctx, keyFresh, []byte("data")); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	n, err := p.cleanExpired(ctx, 1)
	if err != nil {
		t.Fatalf("cleanExpired error: %v", err)
	}
	if n != 1 {
		t.Fatalf("cleanExpired deleted = %d, want 1", n)
	}
	if _, err := os.Stat(filepath.Join(dir, keyOld)); !os.IsNotExist(err) {
		t.Fatalf("expired file still exists after clean")
	}
	if _, err := os.Stat(filepath.Join(dir, keyFresh)); err != nil {
		t.Fatalf("fresh file should be kept: %v", err)
	}

	// 保留期 0（未启用）时不删、不起清理协程
	dir0 := t.TempDir()
	p0 := NewLocalStorageProvider(&LocalConfig{Dir: dir0})
	defer p0.Close()
	store0 := NewImageStore(p0)
	key := "a/x.png"
	if err := store0.Save(ctx, key, []byte("data")); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if err := os.Chtimes(filepath.Join(dir0, key), old, old); err != nil {
		t.Fatalf("Chtimes error: %v", err)
	}
	n0, err := p0.cleanExpired(ctx, 0)
	if err != nil {
		t.Fatalf("cleanExpired error: %v", err)
	}
	if n0 != 0 {
		t.Fatalf("cleanExpired deleted = %d, want 0 (retention disabled)", n0)
	}
}

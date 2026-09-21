package storagex

import (
	"bytes"
	"context"
	"mime"
	"path/filepath"
)

// closer 关闭能力（provider 可选实现：local 停止保留期清理协程）。
type closer interface {
	Close() error
}

// ImageStore 图像存储接口（scan 系统图像存取抽象，local/minio 双后端统一）。
// 消费方仅依赖本接口：Save/Get/URL 存取图像；Delete 删除；RetentionDays 读取保留期；Close 停止后台任务。
type ImageStore interface {
	// Save 保存图像到存储（按 key 落位，同一 key 幂等覆盖）
	Save(ctx context.Context, key string, data []byte) error
	// Get 读取图像全量内容；对象不存在 → ErrNotFound
	Get(ctx context.Context, key string) ([]byte, error)
	// URL 返回图像对外访问 URL（按 provider 配置推导，见 StorageProvider.AccessURL）
	URL(key string) string
	// Delete 删除图像；对象不存在视为成功（幂等）
	Delete(ctx context.Context, key string) error
	// Close 关闭存储，停止 provider 内部后台任务（如 local 保留期清理协程）
	Close() error
}

// New 创建图像存储：cfg 选择后端（local/minio/oss/cos/qiniu），opts 声明启用的行为（保留期等）。
// 行为由后端 provider 构造时装配并持有；ImageStore 按 provider 能力委托保留期/清理。
func New(cfg *StorageConfig, opts ...Option) (ImageStore, error) {
	provider, err := NewStorageProvider(cfg, opts...)
	if err != nil {
		return nil, err
	}
	return &storageImageStore{provider: provider}, nil
}

// storageImageStore 包装 StorageProvider 的 ImageStore 实现。
type storageImageStore struct {
	provider StorageProvider
}

// NewImageStore 创建基于 StorageProvider 的图像存储。
// 访问 URL 由 provider 按自身配置推导（local → 网关静态路径；minio → {endpoint}/{bucket}/key），无需额外配置。
func NewImageStore(provider StorageProvider) ImageStore {
	return &storageImageStore{provider: provider}
}

// Save 保存图像（按 key 扩展名推断 Content-Type）。
func (s *storageImageStore) Save(ctx context.Context, key string, data []byte) error {
	_, err := s.provider.Upload(ctx, bytes.NewReader(data), key,
		WithFileKey(key), WithContentType(contentTypeFor(key)))
	return err
}

// Get 读取图像全量内容（不存在 → ErrNotFound）。
func (s *storageImageStore) Get(ctx context.Context, key string) ([]byte, error) {
	return s.provider.Download(ctx, key)
}

// URL 返回图像对外访问 URL（委托 provider 按配置推导）。
func (s *storageImageStore) URL(key string) string {
	return s.provider.AccessURL(key)
}

// Delete 删除图像（不存在视为成功，幂等）。
func (s *storageImageStore) Delete(ctx context.Context, key string) error {
	return s.provider.Delete(ctx, key)
}

// Close 关闭存储，停止 provider 内部后台任务（provider 支持关闭时委托；否则空操作）。
func (s *storageImageStore) Close() error {
	if c, ok := s.provider.(closer); ok {
		return c.Close()
	}
	return nil
}

// contentTypeFor 按 key 扩展名推断 MIME 类型（未知 → application/octet-stream）。
func contentTypeFor(key string) string {
	if ct := mime.TypeByExtension(filepath.Ext(key)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

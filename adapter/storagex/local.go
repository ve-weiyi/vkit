package storagex

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// localCleanInterval 保留期清理间隔。
const localCleanInterval = time.Hour

// LocalStorageProvider 本地存储服务提供商（用于开发和测试环境）
type LocalStorageProvider struct {
	cfg *LocalConfig

	stop      chan struct{} // 停止清理协程信号（retention > 0 时创建）
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// NewLocalStorageProvider 创建本地存储服务提供商实例
// opts 声明保留期行为：WithRetentionDays 启用保留期并启动清理协程，按间隔扫目录自删过期文件。
// 存储目录创建失败返回 error（目录不可写时落图必然失败，不该等首次使用时才暴露）。
func NewLocalStorageProvider(cfg *LocalConfig, opts ...Option) (*LocalStorageProvider, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	// 确保存储目录存在
	if cfg.Dir == "" {
		cfg.Dir = "./runtime/uploads"
	}
	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create local storage dir: %w", err)
	}

	p := &LocalStorageProvider{
		cfg: cfg,
	}
	if o.retentionDays > 0 {
		p.startCleanup(o.retentionDays)
	}
	return p, nil
}

// startCleanup 启动保留期清理协程：按间隔周期扫目录删除过期文件。
func (p *LocalStorageProvider) startCleanup(days int) {
	p.stop = make(chan struct{})
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(localCleanInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_, _ = p.cleanExpired(context.Background(), days)
			case <-p.stop:
				return
			}
		}
	}()
}

// Close 停止保留期清理协程（幂等）。
func (p *LocalStorageProvider) Close() error {
	if p.stop != nil {
		p.closeOnce.Do(func() { close(p.stop) })
	}
	p.wg.Wait()
	return nil
}

// Upload 服务端上传文件到本地
func (p *LocalStorageProvider) Upload(ctx context.Context, file io.Reader, name string, opts ...UploadOption) (*UploadResult, error) {
	o := resolveOpts(opts, GenerateFileKey(name))

	// 构建完整的文件路径
	fullPath := filepath.Join(p.cfg.Dir, o.FileKey)

	// 确保目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// 创建文件
	dst, err := os.Create(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// 复制文件内容
	if _, err := io.Copy(dst, file); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &UploadResult{AccessURL: p.AccessURL(o.FileKey), FileKey: o.FileKey}, nil
}

// Download 按 key 下载本地文件内容（不存在 → ErrNotFound）
func (p *LocalStorageProvider) Download(ctx context.Context, fileKey string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(p.cfg.Dir, fileKey))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return data, nil
}

// UploadToken 获取上传凭证（本地存储不需要凭证）
func (p *LocalStorageProvider) UploadToken(ctx context.Context, name string, expire time.Duration, opts ...UploadOption) (*UploadTokenResult, error) {
	o := resolveOpts(opts, GenerateFileKey(name))

	// 本地存储的上传URL就是服务端的上传接口
	uploadURL := fmt.Sprintf("%s/api/upload", p.cfg.Endpoint)
	if p.cfg.Endpoint == "" {
		uploadURL = "/api/upload"
	}

	// 访问URL
	accessURL := p.AccessURL(o.FileKey)

	return &UploadTokenResult{
		UploadURL: uploadURL,
		Token:     uuid.New().String(), // 简单的token，实际应该由业务层生成
		FileKey:   o.FileKey,
		AccessURL: accessURL,
		ExpireAt:  time.Now().Add(expire),
		ExtraData: map[string]string{
			"provider": ProviderLocal,
		},
	}, nil
}

// SignURL 生成文件签名访问URL（本地存储返回普通访问路径）
func (p *LocalStorageProvider) SignURL(ctx context.Context, fileKey string, expire time.Duration) (string, error) {
	// 本地存储不支持过期时间，直接返回访问URL
	return p.AccessURL(fileKey), nil
}

// Stat 获取文件元信息
func (p *LocalStorageProvider) Stat(ctx context.Context, fileKey string) (*FileStat, error) {
	fullPath := filepath.Join(p.cfg.Dir, fileKey)
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	return &FileStat{
		FileKey:      fileKey,
		FileName:     info.Name(),
		FileSize:     info.Size(),
		FileURL:      p.AccessURL(fileKey),
		LastModified: info.ModTime(),
	}, nil
}

// List 列举本地文件
func (p *LocalStorageProvider) List(ctx context.Context, prefix string, marker string, limit int) (*ListResult, error) {
	rootDir := filepath.Join(p.cfg.Dir, prefix)
	var files []*FileStat
	var lastPath string
	pastMarker := marker == ""

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(p.cfg.Dir, path)
		if relPath == "." {
			return nil
		}
		if !pastMarker {
			if relPath == marker {
				pastMarker = true
			}
			return nil
		}
		files = append(files, &FileStat{
			IsDir:        info.IsDir(),
			FileKey:      relPath,
			FileName:     info.Name(),
			FileSize:     info.Size(),
			FileURL:      p.AccessURL(relPath),
			LastModified: info.ModTime(),
		})
		lastPath = relPath
		if len(files) >= limit {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	nextMarker := ""
	if len(files) >= limit {
		nextMarker = lastPath
	}
	return &ListResult{Files: files, NextMarker: nextMarker}, nil
}

// Delete 删除本地文件
func (p *LocalStorageProvider) Delete(ctx context.Context, fileKey string) error {
	// 构建完整的文件路径
	fullPath := filepath.Join(p.cfg.Dir, fileKey)

	// 删除文件
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// cleanExpired 清理过期文件：扫描存储目录，删除 modtime 早于保留期截止的文件（含派生缩略图等），返回删除数。
func (p *LocalStorageProvider) cleanExpired(ctx context.Context, days int) (int, error) {
	if days <= 0 {
		return 0, nil
	}
	cutoff := time.Now().AddDate(0, 0, -days)

	deleted := 0
	var firstErr error
	err := filepath.Walk(p.cfg.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// 取消上下文时中止（删除操作幂等，部分删除可接受）
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if info.ModTime().Before(cutoff) {
			if rerr := os.Remove(path); rerr != nil {
				if !os.IsNotExist(rerr) && firstErr == nil {
					firstErr = rerr
				}
			} else {
				deleted++
			}
		}
		return nil
	})
	if err != nil && firstErr == nil {
		firstErr = err
	}
	return deleted, firstErr
}

// ProviderName 获取服务商名称
func (p *LocalStorageProvider) ProviderName() string {
	return ProviderLocal
}

// AccessURL 构建访问URL
func (p *LocalStorageProvider) AccessURL(fileKey string) string {
	// 如果配置了CDN域名，使用CDN域名
	if p.cfg.CDNDomain != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(p.cfg.CDNDomain, "/"), fileKey)
	}

	// 如果配置了Endpoint，使用Endpoint
	if p.cfg.Endpoint != "" {
		return fmt.Sprintf("%s/static/%s", strings.TrimRight(p.cfg.Endpoint, "/"), fileKey)
	}

	// 默认返回相对路径
	return fmt.Sprintf("/static/%s", fileKey)
}

package storagex

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LocalStorageProvider 本地存储服务提供商（用于开发和测试环境）
type LocalStorageProvider struct {
	config *StorageConfig
}

// NewLocalStorageProvider 创建本地存储服务提供商实例
func NewLocalStorageProvider(config *StorageConfig) *LocalStorageProvider {
	// 确保存储目录存在
	if config.BasePath == "" {
		config.BasePath = "./uploads"
	}
	os.MkdirAll(config.BasePath, 0755)

	return &LocalStorageProvider{
		config: config,
	}
}

// GetUploadToken 获取上传凭证（本地存储不需要凭证）
func (p *LocalStorageProvider) GetUploadToken(ctx context.Context, filename string, expireSeconds int) (*UploadToken, error) {
	// 生成唯一文件名
	fileKey := p.generateFileKey(filename)

	// 本地存储的上传URL就是服务端的上传接口
	uploadURL := fmt.Sprintf("%s/api/upload", p.config.Endpoint)
	if p.config.Endpoint == "" {
		uploadURL = "/api/upload"
	}

	// 访问URL
	accessURL := p.buildAccessURL(fileKey)

	return &UploadToken{
		UploadURL: uploadURL,
		Token:     uuid.New().String(), // 简单的token，实际应该由业务层生成
		FileKey:   fileKey,
		AccessURL: accessURL,
		ExpireAt:  time.Now().Add(time.Duration(expireSeconds) * time.Second),
		ExtraData: map[string]string{
			"provider": "local",
		},
	}, nil
}

// Upload 服务端上传文件到本地
func (p *LocalStorageProvider) Upload(ctx context.Context, file io.Reader, filename string) (string, error) {
	// 生成唯一文件名
	fileKey := p.generateFileKey(filename)

	// 构建完整的文件路径
	fullPath := filepath.Join(p.config.BasePath, fileKey)

	// 确保目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// 创建文件
	dst, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// 复制文件内容
	if _, err := io.Copy(dst, file); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// 返回访问URL
	return p.buildAccessURL(fileKey), nil
}

// Delete 删除本地文件
func (p *LocalStorageProvider) Delete(ctx context.Context, fileURL string) error {
	// 从URL中提取文件Key
	fileKey := p.extractFileKey(fileURL)

	// 构建完整的文件路径
	fullPath := filepath.Join(p.config.BasePath, fileKey)

	// 删除文件
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// GetAccessURL 获取文件访问URL（本地存储返回相对路径或HTTP路径）
func (p *LocalStorageProvider) GetAccessURL(ctx context.Context, fileKey string, expireSeconds int) (string, error) {
	// 本地存储不支持过期时间，直接返回访问URL
	return p.buildAccessURL(fileKey), nil
}

// ListFiles 列举本地文件
func (p *LocalStorageProvider) ListFiles(ctx context.Context, prefix string, limit int) ([]*FileInfo, error) {
	rootDir := filepath.Join(p.config.BasePath, prefix)
	var files []*FileInfo

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(p.config.BasePath, path)
		files = append(files, &FileInfo{
			IsDir:    info.IsDir(),
			FilePath: relPath,
			FileName: info.Name(),
			FileType: filepath.Ext(info.Name()),
			FileSize: info.Size(),
			FileURL:  p.buildAccessURL(relPath),
			UpTime:   info.ModTime(),
		})
		if len(files) >= limit {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	return files, nil
}

// GetProviderName 获取服务商名称
func (p *LocalStorageProvider) GetProviderName() string {
	return "local"
}

// generateFileKey 生成唯一的文件Key
func (p *LocalStorageProvider) generateFileKey(filename string) string {
	return GenerateFileKey(p.config.BasePath, filename)
}

// buildAccessURL 构建访问URL
func (p *LocalStorageProvider) buildAccessURL(fileKey string) string {
	// 如果配置了CDN域名，使用CDN域名
	if p.config.CDNDomain != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(p.config.CDNDomain, "/"), fileKey)
	}

	// 如果配置了Endpoint，使用Endpoint
	if p.config.Endpoint != "" {
		return fmt.Sprintf("%s/static/%s", strings.TrimRight(p.config.Endpoint, "/"), fileKey)
	}

	// 默认返回相对路径
	return fmt.Sprintf("/static/%s", fileKey)
}

// extractFileKey 从URL中提取文件Key
func (p *LocalStorageProvider) extractFileKey(fileURL string) string {
	// 移除域名部分
	if strings.Contains(fileURL, "://") {
		parts := strings.SplitN(fileURL, "://", 2)
		if len(parts) == 2 {
			fileURL = parts[1]
			// 移除域名
			if idx := strings.Index(fileURL, "/"); idx != -1 {
				fileURL = fileURL[idx+1:]
			}
		}
	}

	// 移除 /static/ 前缀
	fileURL = strings.TrimPrefix(fileURL, "/static/")
	fileURL = strings.TrimPrefix(fileURL, "static/")

	return fileURL
}

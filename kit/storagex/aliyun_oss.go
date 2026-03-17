package storagex

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// AliyunOSSProvider 阿里云OSS存储服务提供商
type AliyunOSSProvider struct {
	config *StorageConfig
	client *oss.Client
	bucket *oss.Bucket
}

// NewAliyunOSSProvider 创建阿里云OSS存储服务提供商实例
func NewAliyunOSSProvider(config *StorageConfig) *AliyunOSSProvider {
	// 创建OSS客户端
	client, err := oss.New(config.Endpoint, config.AccessKey, config.SecretKey)
	if err != nil {
		panic(fmt.Sprintf("failed to create aliyun oss client: %v", err))
	}

	// 获取存储桶
	bucket, err := client.Bucket(config.Bucket)
	if err != nil {
		panic(fmt.Sprintf("failed to get aliyun oss bucket: %v", err))
	}

	return &AliyunOSSProvider{
		config: config,
		client: client,
		bucket: bucket,
	}
}

// GetUploadToken 获取上传凭证
func (p *AliyunOSSProvider) GetUploadToken(ctx context.Context, filename string, expireSeconds int) (*UploadToken, error) {
	// 生成唯一文件Key
	fileKey := p.generateFileKey(filename)

	// 生成签名URL（用于前端直传）
	expireTime := time.Duration(expireSeconds) * time.Second
	signedURL, err := p.bucket.SignURL(fileKey, oss.HTTPPut, int64(expireTime.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("failed to generate signed url: %w", err)
	}

	// 构建访问URL
	accessURL := p.buildAccessURL(fileKey)

	return &UploadToken{
		UploadURL: signedURL,
		Token:     "",
		FileKey:   fileKey,
		AccessURL: accessURL,
		ExpireAt:  time.Now().Add(expireTime),
		ExtraData: map[string]string{
			"provider": "aliyun",
			"bucket":   p.config.Bucket,
		},
	}, nil
}

// Upload 服务端上传文件
func (p *AliyunOSSProvider) Upload(ctx context.Context, file io.Reader, filename string) (string, error) {
	// 生成唯一文件Key
	fileKey := p.generateFileKey(filename)

	// 上传文件
	err := p.bucket.PutObject(fileKey, file)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// 返回访问URL
	return p.buildAccessURL(fileKey), nil
}

// Delete 删除文件
func (p *AliyunOSSProvider) Delete(ctx context.Context, fileURL string) error {
	// 从URL中提取文件Key
	fileKey := p.extractFileKey(fileURL)

	// 删除文件
	err := p.bucket.DeleteObject(fileKey)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// GetAccessURL 获取文件访问URL
func (p *AliyunOSSProvider) GetAccessURL(ctx context.Context, fileKey string, expireSeconds int) (string, error) {
	// 如果是私有存储，生成签名URL
	if p.config.IsPrivate {
		expireTime := int64(expireSeconds)
		if expireTime == 0 {
			expireTime = 3600 // 默认1小时
		}
		signedURL, err := p.bucket.SignURL(fileKey, oss.HTTPGet, expireTime)
		if err != nil {
			return "", fmt.Errorf("failed to generate signed url: %w", err)
		}
		return signedURL, nil
	}

	// 公共存储，直接返回访问URL
	return p.buildAccessURL(fileKey), nil
}

// ListFiles 列举文件
func (p *AliyunOSSProvider) ListFiles(ctx context.Context, prefix string, limit int) ([]*FileInfo, error) {
	result, err := p.bucket.ListObjectsV2(oss.Prefix(prefix), oss.MaxKeys(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	files := make([]*FileInfo, 0, len(result.Objects))
	for _, obj := range result.Objects {
		files = append(files, &FileInfo{
			IsDir:    false,
			FilePath: obj.Key,
			FileName: filepath.Base(obj.Key),
			FileType: filepath.Ext(obj.Key),
			FileSize: obj.Size,
			FileURL:  p.buildAccessURL(obj.Key),
			UpTime:   obj.LastModified,
		})
	}
	return files, nil
}

// GetProviderName 获取服务商名称
func (p *AliyunOSSProvider) GetProviderName() string {
	return "aliyun"
}

// generateFileKey 生成唯一的文件Key
func (p *AliyunOSSProvider) generateFileKey(filename string) string {
	return GenerateFileKey(p.config.BasePath, filename)
}

// buildAccessURL 构建访问URL
func (p *AliyunOSSProvider) buildAccessURL(fileKey string) string {
	// 如果配置了CDN域名，使用CDN域名
	if p.config.CDNDomain != "" {
		return buildURLWithDomain(p.config.CDNDomain, fileKey)
	}

	// 使用OSS默认域名
	return fmt.Sprintf("https://%s.%s/%s", p.config.Bucket, p.config.Endpoint, fileKey)
}

// extractFileKey 从URL中提取文件Key
func (p *AliyunOSSProvider) extractFileKey(fileURL string) string {
	// 移除协议和域名部分
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

	// 移除查询参数
	if idx := strings.Index(fileURL, "?"); idx != -1 {
		fileURL = fileURL[:idx]
	}

	return fileURL
}

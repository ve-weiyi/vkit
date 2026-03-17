package storagex

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
)

// TencentCOSProvider 腾讯云COS存储服务提供商
type TencentCOSProvider struct {
	config *StorageConfig
	client *cos.Client
}

// NewTencentCOSProvider 创建腾讯云COS存储服务提供商实例
func NewTencentCOSProvider(config *StorageConfig) *TencentCOSProvider {
	// 构建存储桶URL
	bucketURL := fmt.Sprintf("https://%s.cos.%s.myqcloud.com", config.Bucket, config.Region)
	u, _ := url.Parse(bucketURL)

	// 创建COS客户端
	b := &cos.BaseURL{BucketURL: u}
	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  config.AccessKey,
			SecretKey: config.SecretKey,
		},
	})

	return &TencentCOSProvider{
		config: config,
		client: client,
	}
}

// GetUploadToken 获取上传凭证
func (p *TencentCOSProvider) GetUploadToken(ctx context.Context, filename string, expireSeconds int) (*UploadToken, error) {
	// 生成唯一文件Key
	fileKey := p.generateFileKey(filename)

	// 生成预签名URL（用于前端直传）
	presignedURL, err := p.client.Object.GetPresignedURL(
		ctx,
		http.MethodPut,
		fileKey,
		p.config.AccessKey,
		p.config.SecretKey,
		time.Duration(expireSeconds)*time.Second,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned url: %w", err)
	}

	// 构建访问URL
	accessURL := p.buildAccessURL(fileKey)

	return &UploadToken{
		UploadURL: presignedURL.String(),
		Token:     "",
		FileKey:   fileKey,
		AccessURL: accessURL,
		ExpireAt:  time.Now().Add(time.Duration(expireSeconds) * time.Second),
		ExtraData: map[string]string{
			"provider": "tencent",
			"bucket":   p.config.Bucket,
			"region":   p.config.Region,
		},
	}, nil
}

// Upload 服务端上传文件
func (p *TencentCOSProvider) Upload(ctx context.Context, file io.Reader, filename string) (string, error) {
	// 生成唯一文件Key
	fileKey := p.generateFileKey(filename)

	// 上传文件
	_, err := p.client.Object.Put(ctx, fileKey, file, nil)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// 返回访问URL
	return p.buildAccessURL(fileKey), nil
}

// Delete 删除文件
func (p *TencentCOSProvider) Delete(ctx context.Context, fileURL string) error {
	// 从URL中提取文件Key
	fileKey := p.extractFileKey(fileURL)

	// 删除文件
	_, err := p.client.Object.Delete(ctx, fileKey)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// GetAccessURL 获取文件访问URL
func (p *TencentCOSProvider) GetAccessURL(ctx context.Context, fileKey string, expireSeconds int) (string, error) {
	// 如果是私有存储，生成预签名URL
	if p.config.IsPrivate {
		expireTime := time.Duration(expireSeconds) * time.Second
		if expireTime == 0 {
			expireTime = time.Hour // 默认1小时
		}
		presignedURL, err := p.client.Object.GetPresignedURL(
			ctx,
			http.MethodGet,
			fileKey,
			p.config.AccessKey,
			p.config.SecretKey,
			expireTime,
			nil,
		)
		if err != nil {
			return "", fmt.Errorf("failed to generate presigned url: %w", err)
		}
		return presignedURL.String(), nil
	}

	// 公共存储，直接返回访问URL
	return p.buildAccessURL(fileKey), nil
}

// ListFiles 列举文件
func (p *TencentCOSProvider) ListFiles(ctx context.Context, prefix string, limit int) ([]*FileInfo, error) {
	result, _, err := p.client.Bucket.Get(ctx, &cos.BucketGetOptions{
		Prefix:  prefix,
		MaxKeys: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	files := make([]*FileInfo, 0, len(result.Contents))
	for _, obj := range result.Contents {
		lastModified, _ := time.Parse(time.RFC3339, obj.LastModified)
		files = append(files, &FileInfo{
			IsDir:    false,
			FilePath: obj.Key,
			FileName: filepath.Base(obj.Key),
			FileType: filepath.Ext(obj.Key),
			FileSize: int64(obj.Size),
			FileURL:  p.buildAccessURL(obj.Key),
			UpTime:   lastModified,
		})
	}
	return files, nil
}

// GetProviderName 获取服务商名称
func (p *TencentCOSProvider) GetProviderName() string {
	return "tencent"
}

// generateFileKey 生成唯一的文件Key
func (p *TencentCOSProvider) generateFileKey(filename string) string {
	return GenerateFileKey(p.config.BasePath, filename)
}

// buildAccessURL 构建访问URL
func (p *TencentCOSProvider) buildAccessURL(fileKey string) string {
	// 如果配置了CDN域名，使用CDN域名
	if p.config.CDNDomain != "" {
		return buildURLWithDomain(p.config.CDNDomain, fileKey)
	}

	// 使用COS默认域名
	return fmt.Sprintf("https://%s.cos.%s.myqcloud.com/%s", p.config.Bucket, p.config.Region, fileKey)
}

// extractFileKey 从URL中提取文件Key
func (p *TencentCOSProvider) extractFileKey(fileURL string) string {
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

package storagex

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// AliyunOSSProvider 阿里云OSS存储服务提供商
type AliyunOSSProvider struct {
	cfg    *OSSConfig
	client *oss.Client
	bucket *oss.Bucket
}

// NewAliyunOSSProvider 创建阿里云OSS存储服务提供商实例
// opts 忽略：该后端暂未装配对应行为（生命周期等后续按能力实现）。
func NewAliyunOSSProvider(cfg *OSSConfig, opts ...Option) (*AliyunOSSProvider, error) {
	// 创建OSS客户端
	client, err := oss.New(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create aliyun oss client: %w", err)
	}

	// 获取存储桶
	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to get aliyun oss bucket: %w", err)
	}

	return &AliyunOSSProvider{
		cfg:    cfg,
		client: client,
		bucket: bucket,
	}, nil
}

// Upload 服务端上传文件
func (p *AliyunOSSProvider) Upload(ctx context.Context, file io.Reader, name string, opts ...UploadOption) (*UploadResult, error) {
	o := resolveOpts(opts, GenerateFileKey(name))

	// 上传文件
	err := p.bucket.PutObject(o.FileKey, file)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	return &UploadResult{AccessURL: p.AccessURL(o.FileKey), FileKey: o.FileKey}, nil
}

// Download 按 key 下载对象内容（不存在 → ErrNotFound）
func (p *AliyunOSSProvider) Download(ctx context.Context, fileKey string) ([]byte, error) {
	obj, err := p.bucket.GetObject(fileKey)
	if err != nil {
		if ossErr, ok := err.(oss.ServiceError); ok && ossErr.Code == "NoSuchKey" {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer obj.Close()

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to read object: %w", err)
	}
	return data, nil
}

// UploadToken 获取上传凭证
func (p *AliyunOSSProvider) UploadToken(ctx context.Context, name string, expire time.Duration, opts ...UploadOption) (*UploadTokenResult, error) {
	o := resolveOpts(opts, GenerateFileKey(name))

	// 生成签名URL（用于前端直传）
	signedURL, err := p.bucket.SignURL(o.FileKey, oss.HTTPPut, int64(expire.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("failed to generate signed url: %w", err)
	}

	// 构建访问URL
	accessURL := p.AccessURL(o.FileKey)

	return &UploadTokenResult{
		UploadURL: signedURL,
		Token:     "",
		FileKey:   o.FileKey,
		AccessURL: accessURL,
		ExpireAt:  time.Now().Add(expire),
		ExtraData: map[string]string{
			"provider": ProviderAliyun,
			"bucket":   p.cfg.Bucket,
		},
	}, nil
}

// SignURL 生成文件签名访问URL
func (p *AliyunOSSProvider) SignURL(ctx context.Context, fileKey string, expire time.Duration) (string, error) {
	// 如果是私有存储，生成签名URL
	if p.cfg.IsPrivate {
		if expire <= 0 {
			expire = time.Hour
		}
		signedURL, err := p.bucket.SignURL(fileKey, oss.HTTPGet, int64(expire.Seconds()))
		if err != nil {
			return "", fmt.Errorf("failed to generate signed url: %w", err)
		}
		return signedURL, nil
	}

	// 公共存储，直接返回访问URL
	return p.AccessURL(fileKey), nil
}

// Stat 获取文件元信息
func (p *AliyunOSSProvider) Stat(ctx context.Context, fileKey string) (*FileStat, error) {
	headers, err := p.bucket.GetObjectMeta(fileKey)
	if err != nil {
		return nil, fmt.Errorf("head object: %w", err)
	}
	var size int64
	if cl := headers.Get("Content-Length"); cl != "" {
		fmt.Sscanf(cl, "%d", &size)
	}
	modTime, _ := time.Parse(time.RFC1123, headers.Get("Last-Modified"))
	return &FileStat{
		FileKey:      fileKey,
		FileName:     filepath.Base(fileKey),
		FileSize:     size,
		ContentType:  headers.Get("Content-Type"),
		FileURL:      p.AccessURL(fileKey),
		LastModified: modTime,
	}, nil
}

// List 列举文件
func (p *AliyunOSSProvider) List(ctx context.Context, prefix string, marker string, limit int) (*ListResult, error) {
	result, err := p.bucket.ListObjectsV2(oss.Prefix(prefix), oss.StartAfter(marker), oss.MaxKeys(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	files := make([]*FileStat, 0, len(result.Objects))
	for _, obj := range result.Objects {
		files = append(files, &FileStat{
			IsDir:        false,
			FileKey:      obj.Key,
			FileName:     filepath.Base(obj.Key),
			FileSize:     obj.Size,
			FileURL:      p.AccessURL(obj.Key),
			LastModified: obj.LastModified,
		})
	}
	nextMarker := ""
	if len(files) >= limit && len(files) > 0 {
		nextMarker = files[len(files)-1].FileKey
	}
	return &ListResult{Files: files, NextMarker: nextMarker}, nil
}

// Delete 删除文件
func (p *AliyunOSSProvider) Delete(ctx context.Context, fileKey string) error {
	err := p.bucket.DeleteObject(fileKey)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// ProviderName 获取服务商名称
func (p *AliyunOSSProvider) ProviderName() string {
	return ProviderAliyun
}

// AccessURL 构建访问URL
func (p *AliyunOSSProvider) AccessURL(fileKey string) string {
	// 如果配置了CDN域名，使用CDN域名
	if p.cfg.CDNDomain != "" {
		return buildURLWithDomain(p.cfg.CDNDomain, fileKey)
	}

	// 使用OSS默认域名
	return fmt.Sprintf("https://%s.%s/%s", p.cfg.Bucket, p.cfg.Endpoint, fileKey)
}

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
	cfg    *COSConfig
	client *cos.Client
}

// NewTencentCOSProvider 创建腾讯云COS存储服务提供商实例
// opts 忽略：该后端暂未装配对应行为（生命周期等后续按能力实现）。
func NewTencentCOSProvider(cfg *COSConfig, opts ...Option) *TencentCOSProvider {
	u, _ := url.Parse(cfg.Endpoint)

	// 创建COS客户端
	b := &cos.BaseURL{BucketURL: u}
	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.SecretID,
			SecretKey: cfg.SecretKey,
		},
	})

	return &TencentCOSProvider{
		cfg:    cfg,
		client: client,
	}
}

// Upload 服务端上传文件
func (p *TencentCOSProvider) Upload(ctx context.Context, file io.Reader, name string, opts ...UploadOption) (*UploadResult, error) {
	o := resolveOpts(opts, GenerateFileKey(name))

	// 上传文件
	_, err := p.client.Object.Put(ctx, o.FileKey, file, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	return &UploadResult{AccessURL: p.AccessURL(o.FileKey), FileKey: o.FileKey}, nil
}

// Download 按 key 下载对象内容（不存在 → ErrNotFound）
func (p *TencentCOSProvider) Download(ctx context.Context, fileKey string) ([]byte, error) {
	resp, err := p.client.Object.Get(ctx, fileKey, nil)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object: %w", err)
	}
	return data, nil
}

// UploadToken 获取上传凭证
func (p *TencentCOSProvider) UploadToken(ctx context.Context, name string, expire time.Duration, opts ...UploadOption) (*UploadTokenResult, error) {
	o := resolveOpts(opts, GenerateFileKey(name))

	// 生成预签名URL（用于前端直传）
	presignedURL, err := p.client.Object.GetPresignedURL(
		ctx,
		http.MethodPut,
		o.FileKey,
		p.cfg.SecretID,
		p.cfg.SecretKey,
		expire,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned url: %w", err)
	}

	// 构建访问URL
	accessURL := p.AccessURL(o.FileKey)

	return &UploadTokenResult{
		UploadURL: presignedURL.String(),
		Token:     "",
		FileKey:   o.FileKey,
		AccessURL: accessURL,
		ExpireAt:  time.Now().Add(expire),
		ExtraData: map[string]string{
			"provider": ProviderTencent,
			"bucket":   p.cfg.Bucket,
		},
	}, nil
}

// SignURL 生成文件签名访问URL
func (p *TencentCOSProvider) SignURL(ctx context.Context, fileKey string, expire time.Duration) (string, error) {
	// 如果是私有存储，生成预签名URL
	if p.cfg.IsPrivate {
		if expire == 0 {
			expire = time.Hour // 默认1小时
		}
		presignedURL, err := p.client.Object.GetPresignedURL(
			ctx,
			http.MethodGet,
			fileKey,
			p.cfg.SecretID,
			p.cfg.SecretKey,
			expire,
			nil,
		)
		if err != nil {
			return "", fmt.Errorf("failed to generate presigned url: %w", err)
		}
		return presignedURL.String(), nil
	}

	// 公共存储，直接返回访问URL
	return p.AccessURL(fileKey), nil
}

// Stat 获取文件元信息
func (p *TencentCOSProvider) Stat(ctx context.Context, fileKey string) (*FileStat, error) {
	resp, err := p.client.Object.Head(ctx, fileKey, nil)
	if err != nil {
		return nil, fmt.Errorf("head object: %w", err)
	}
	modTime, _ := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
	return &FileStat{
		FileKey:      fileKey,
		FileName:     filepath.Base(fileKey),
		FileSize:     resp.ContentLength,
		ContentType:  resp.Header.Get("Content-Type"),
		FileURL:      p.AccessURL(fileKey),
		LastModified: modTime,
	}, nil
}

// List 列举文件
func (p *TencentCOSProvider) List(ctx context.Context, prefix string, marker string, limit int) (*ListResult, error) {
	result, _, err := p.client.Bucket.Get(ctx, &cos.BucketGetOptions{
		Prefix:  prefix,
		Marker:  marker,
		MaxKeys: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	files := make([]*FileStat, 0, len(result.Contents))
	for _, obj := range result.Contents {
		lastModified, _ := time.Parse(time.RFC3339, obj.LastModified)
		files = append(files, &FileStat{
			IsDir:        false,
			FileKey:      obj.Key,
			FileName:     filepath.Base(obj.Key),
			FileSize:     obj.Size,
			FileURL:      p.AccessURL(obj.Key),
			LastModified: lastModified,
		})
	}
	return &ListResult{Files: files, NextMarker: result.NextMarker}, nil
}

// Delete 删除文件
func (p *TencentCOSProvider) Delete(ctx context.Context, fileKey string) error {
	_, err := p.client.Object.Delete(ctx, fileKey)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// ProviderName 获取服务商名称
func (p *TencentCOSProvider) ProviderName() string {
	return ProviderTencent
}

// AccessURL 构建访问URL
func (p *TencentCOSProvider) AccessURL(fileKey string) string {
	// 如果配置了CDN域名，使用CDN域名
	if p.cfg.CDNDomain != "" {
		return buildURLWithDomain(p.cfg.CDNDomain, fileKey)
	}

	return fmt.Sprintf("%s/%s", strings.TrimRight(p.cfg.Endpoint, "/"), fileKey)
}

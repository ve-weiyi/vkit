package storagex

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
)

// MinioProvider MinIO 存储服务提供商
type MinioProvider struct {
	cfg    *MinioConfig
	client *minio.Client
}

// NewMinioProvider 创建 MinIO 存储服务提供商实例
// opts 声明启用的行为：WithRetentionDays 下发桶生命周期规则（option 优先，cfg.Lifecycle 兜底）。
func NewMinioProvider(cfg *MinioConfig, opts ...Option) (*MinioProvider, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	// 保留期：option 声明的保留期优先，未声明时回退配置 Lifecycle
	days := o.retentionDays
	if days <= 0 && cfg.Lifecycle != nil {
		days = cfg.Lifecycle.ExpireDays
	}

	p := &MinioProvider{
		cfg:    cfg,
		client: client,
	}

	// 确保 Bucket 存在
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check minio bucket: %w", err)
	}
	if !exists {
		err = client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create minio bucket: %w", err)
		}
	}

	// 设置 Bucket 公开读权限（用于 CDN 访问）
	if err := p.ensurePublicPolicy(ctx); err != nil {
		return nil, fmt.Errorf("failed to set minio bucket policy: %w", err)
	}

	// 设置生命周期规则（保留期 > 0 时下发）
	if days > 0 {
		if err := p.ensureLifecycle(ctx, days); err != nil {
			return nil, fmt.Errorf("failed to set minio lifecycle: %w", err)
		}
	}

	return p, nil
}

// CleanExpired 清理过期对象：minio 走桶生命周期自动过期，无需客户端清理。
func (p *MinioProvider) CleanExpired(ctx context.Context) (int, error) {
	return 0, nil
}

// ensurePublicPolicy 设置 Bucket 公开读权限（幂等）
func (p *MinioProvider) ensurePublicPolicy(ctx context.Context) error {
	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": {"AWS": ["*"]},
			"Action": ["s3:GetObject"],
			"Resource": ["arn:aws:s3:::%s/*"]
		}]
	}`, p.cfg.Bucket)
	return p.client.SetBucketPolicy(ctx, p.cfg.Bucket, policy)
}

// ensureLifecycle 设置 Bucket 生命周期规则（幂等）
func (p *MinioProvider) ensureLifecycle(ctx context.Context, expireDays int) error {
	config := lifecycle.NewConfiguration()
	config.Rules = []lifecycle.Rule{{
		ID:         "expire-chat-files",
		Status:     "Enabled",
		Expiration: lifecycle.Expiration{Days: lifecycle.ExpirationDays(expireDays)},
	}}
	return p.client.SetBucketLifecycle(ctx, p.cfg.Bucket, config)
}

// Upload 服务端上传文件
func (p *MinioProvider) Upload(ctx context.Context, file io.Reader, name string, opts ...UploadOption) (*UploadResult, error) {
	o := resolveOpts(opts, GenerateFileKey(name))

	_, err := p.client.PutObject(ctx, p.cfg.Bucket, o.FileKey, file, -1,
		minio.PutObjectOptions{ContentType: o.ContentType},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	return &UploadResult{AccessURL: p.AccessURL(o.FileKey), FileKey: o.FileKey}, nil
}

// Download 按 key 下载对象内容（不存在 → ErrNotFound）
func (p *MinioProvider) Download(ctx context.Context, fileKey string) ([]byte, error) {
	obj, err := p.client.GetObject(ctx, p.cfg.Bucket, fileKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer obj.Close()

	data, err := io.ReadAll(obj)
	if err != nil {
		// 对象不存在时 GetObject 在 Read 阶段返回 NoSuchKey
		if resp := minio.ToErrorResponse(err); resp.Code == "NoSuchKey" {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to read object: %w", err)
	}
	return data, nil
}

// UploadToken 获取上传凭证
func (p *MinioProvider) UploadToken(ctx context.Context, name string, expire time.Duration, opts ...UploadOption) (*UploadTokenResult, error) {
	o := resolveOpts(opts, GenerateFileKey(name))

	// Presigned PUT URL
	presignedURL, err := p.client.PresignedPutObject(ctx, p.cfg.Bucket, o.FileKey, expire)
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned url: %w", err)
	}

	accessURL := p.AccessURL(o.FileKey)

	return &UploadTokenResult{
		UploadURL: presignedURL.String(),
		Token:     "",
		FileKey:   o.FileKey,
		AccessURL: accessURL,
		ExpireAt:  time.Now().Add(expire),
		ExtraData: map[string]string{
			"provider": ProviderMinio,
			"bucket":   p.cfg.Bucket,
		},
	}, nil
}

// SignURL 生成文件签名访问URL
func (p *MinioProvider) SignURL(ctx context.Context, fileKey string, expire time.Duration) (string, error) {
	// 若配置了 CDN，构造公开 URL（走 Ingress → MinIO，无需签名）
	if p.cfg.CDNDomain != "" {
		return buildURLWithDomain(p.cfg.CDNDomain, p.cfg.Bucket+"/"+fileKey), nil
	}
	// 否则生成 Presigned URL（开发环境，直连 MinIO）
	if expire <= 0 {
		expire = time.Hour
	}
	u, err := p.client.PresignedGetObject(ctx, p.cfg.Bucket, fileKey, expire, url.Values{})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned url: %w", err)
	}
	return u.String(), nil
}

// Stat 获取文件元信息
func (p *MinioProvider) Stat(ctx context.Context, fileKey string) (*FileStat, error) {
	info, err := p.client.StatObject(ctx, p.cfg.Bucket, fileKey, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("stat object: %w", err)
	}
	return &FileStat{
		FileKey:      fileKey,
		FileName:     filepath.Base(fileKey),
		FileSize:     info.Size,
		FileURL:      p.AccessURL(fileKey),
		LastModified: info.LastModified,
	}, nil
}

// List 列举文件
func (p *MinioProvider) List(ctx context.Context, prefix string, marker string, limit int) (*ListResult, error) {
	objCh := p.client.ListObjects(ctx, p.cfg.Bucket, minio.ListObjectsOptions{
		Prefix:     prefix,
		StartAfter: marker,
		Recursive:  true,
		MaxKeys:    limit,
	})

	var files []*FileStat
	for obj := range objCh {
		if obj.Err != nil {
			return nil, fmt.Errorf("failed to list files: %w", obj.Err)
		}
		files = append(files, &FileStat{
			IsDir:        false,
			FileKey:      obj.Key,
			FileName:     filepath.Base(obj.Key),
			FileSize:     obj.Size,
			FileURL:      p.AccessURL(obj.Key),
			ContentType:  obj.ContentType,
			LastModified: obj.LastModified,
		})
	}
	nextMarker := ""
	if len(files) >= limit {
		nextMarker = files[len(files)-1].FileKey
	}
	return &ListResult{Files: files, NextMarker: nextMarker}, nil
}

// Delete 删除文件
func (p *MinioProvider) Delete(ctx context.Context, fileKey string) error {
	err := p.client.RemoveObject(ctx, p.cfg.Bucket, fileKey, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// ProviderName 获取服务商名称
func (p *MinioProvider) ProviderName() string {
	return ProviderMinio
}

// AccessURL 构建访问URL（无 CDN 时按 Secure 补协议前缀，Endpoint 为 host:port 无 scheme）
func (p *MinioProvider) AccessURL(fileKey string) string {
	if p.cfg.CDNDomain != "" {
		return buildURLWithDomain(p.cfg.CDNDomain, p.cfg.Bucket+"/"+fileKey)
	}
	scheme := "http"
	if p.cfg.Secure {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, p.cfg.Endpoint, p.cfg.Bucket, fileKey)
}

package storagex

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
)

// QiniuKodoProvider 七牛云Kodo存储服务提供商
type QiniuKodoProvider struct {
	cfg *KodoConfig
	mac *qbox.Mac
}

// NewQiniuKodoProvider 创建七牛云Kodo存储服务提供商实例
// opts 忽略：该后端暂未装配对应行为（生命周期等后续按能力实现）。
func NewQiniuKodoProvider(cfg *KodoConfig, opts ...Option) *QiniuKodoProvider {
	// 创建认证对象
	mac := qbox.NewMac(cfg.AccessKey, cfg.SecretKey)

	return &QiniuKodoProvider{
		cfg: cfg,
		mac: mac,
	}
}

// Upload 服务端上传文件
func (p *QiniuKodoProvider) Upload(ctx context.Context, file io.Reader, name string, opts ...UploadOption) (*UploadResult, error) {
	o := resolveOpts(opts, GenerateFileKey(name))

	// 构建上传策略
	putPolicy := storage.PutPolicy{
		Scope: fmt.Sprintf("%s:%s", p.cfg.Bucket, o.FileKey),
	}
	uploadToken := putPolicy.UploadToken(p.mac)

	// 获取上传配置
	cfg := p.getUploadConfig()

	// 创建表单上传对象
	formUploader := storage.NewFormUploader(cfg)
	ret := storage.PutRet{}
	putExtra := storage.PutExtra{}
	// 上传文件
	err := formUploader.Put(ctx, &ret, uploadToken, o.FileKey, file, -1, &putExtra)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	return &UploadResult{AccessURL: p.AccessURL(o.FileKey), FileKey: o.FileKey}, nil
}

// Download 按 key 下载对象内容（不存在 → ErrNotFound）
func (p *QiniuKodoProvider) Download(ctx context.Context, fileKey string) ([]byte, error) {
	// 私有存储经签名 URL，公共存储经公开 URL（对齐 SignURL 语义）
	u, err := p.SignURL(ctx, fileKey, time.Hour)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build download request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download object: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download object: unexpected status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object: %w", err)
	}
	return data, nil
}

// UploadToken 获取上传凭证
func (p *QiniuKodoProvider) UploadToken(ctx context.Context, name string, expire time.Duration, opts ...UploadOption) (*UploadTokenResult, error) {
	o := resolveOpts(opts, GenerateFileKey(name))

	// 构建上传策略
	putPolicy := storage.PutPolicy{
		Scope:   fmt.Sprintf("%s:%s", p.cfg.Bucket, o.FileKey),
		Expires: uint64(expire.Seconds()),
	}

	// 生成上传凭证
	uploadToken := putPolicy.UploadToken(p.mac)

	// 构建上传URL
	uploadURL := p.cfg.Endpoint
	if !strings.HasPrefix(uploadURL, "http") {
		uploadURL = "https://" + uploadURL
	}

	// 构建访问URL
	accessURL := p.AccessURL(o.FileKey)

	return &UploadTokenResult{
		UploadURL: uploadURL,
		Token:     uploadToken,
		FileKey:   o.FileKey,
		AccessURL: accessURL,
		ExpireAt:  time.Now().Add(expire),
		ExtraData: map[string]string{
			"provider": ProviderQiniu,
			"bucket":   p.cfg.Bucket,
		},
	}, nil
}

// SignURL 生成文件签名访问URL
func (p *QiniuKodoProvider) SignURL(ctx context.Context, fileKey string, expire time.Duration) (string, error) {
	if expire <= 0 {
		expire = time.Hour
	}
	// 如果是私有存储，生成签名URL
	if p.cfg.IsPrivate {
		domain := p.cfg.CDNDomain
		if domain == "" {
			domain = fmt.Sprintf("%s.qiniucdn.com", p.cfg.Bucket)
		}
		deadline := time.Now().Add(expire).Unix()
		privateURL := storage.MakePrivateURL(p.mac, domain, p.AccessURL(fileKey), deadline)
		return privateURL, nil
	}

	// 公共存储，直接返回访问URL
	return p.AccessURL(fileKey), nil
}

// Stat 获取文件元信息
func (p *QiniuKodoProvider) Stat(ctx context.Context, fileKey string) (*FileStat, error) {
	bucketManager := storage.NewBucketManager(p.mac, nil)
	info, err := bucketManager.Stat(p.cfg.Bucket, fileKey)
	if err != nil {
		return nil, fmt.Errorf("stat object: %w", err)
	}
	return &FileStat{
		FileKey:      fileKey,
		FileName:     filepath.Base(fileKey),
		FileSize:     info.Fsize,
		ContentType:  info.MimeType,
		FileURL:      p.AccessURL(fileKey),
		LastModified: time.UnixMicro(info.PutTime / 10),
	}, nil
}

// List 列举文件
func (p *QiniuKodoProvider) List(ctx context.Context, prefix string, marker string, limit int) (*ListResult, error) {
	cfg := p.getUploadConfig()
	bucketManager := storage.NewBucketManager(p.mac, cfg)

	entries, prefixes, nextMarker, _, err := bucketManager.ListFiles(p.cfg.Bucket, prefix, "/", marker, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	files := make([]*FileStat, 0, len(prefixes)+len(entries))
	for _, fix := range prefixes {
		files = append(files, &FileStat{
			IsDir:    true,
			FileKey:  fix,
			FileName: filepath.Base(fix),
			FileURL:  p.AccessURL(fix),
		})
	}
	for _, entry := range entries {
		if entry.Fsize == 0 {
			continue
		}
		files = append(files, &FileStat{
			IsDir:        false,
			FileKey:      entry.Key,
			FileName:     filepath.Base(entry.Key),
			FileSize:     entry.Fsize,
			FileURL:      p.AccessURL(entry.Key),
			LastModified: time.UnixMicro(entry.PutTime / 10),
		})
	}
	return &ListResult{Files: files, NextMarker: nextMarker}, nil
}

// Delete 删除文件
func (p *QiniuKodoProvider) Delete(ctx context.Context, fileKey string) error {
	// 获取存储配置
	cfg := p.getUploadConfig()

	// 创建存储管理对象
	bucketManager := storage.NewBucketManager(p.mac, cfg)

	// 删除文件
	err := bucketManager.Delete(p.cfg.Bucket, fileKey)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// ProviderName 获取服务商名称
func (p *QiniuKodoProvider) ProviderName() string {
	return ProviderQiniu
}

// AccessURL 构建访问URL
func (p *QiniuKodoProvider) AccessURL(fileKey string) string {
	// 如果配置了CDN域名，使用CDN域名
	if p.cfg.CDNDomain != "" {
		return buildURLWithDomain(p.cfg.CDNDomain, fileKey)
	}

	// 使用七牛云默认域名（需要在控制台配置）
	// 注意：七牛云必须配置CDN域名才能访问，这里返回一个占位符
	return fmt.Sprintf("https://%s.qiniucdn.com/%s", p.cfg.Bucket, fileKey)
}

// getRegion 获取存储区域配置
func (p *QiniuKodoProvider) getRegion() *storage.Zone {
	// 根据配置的Region返回对应的区域
	switch p.cfg.Region {
	case "z0", "huadong":
		return &storage.ZoneHuadong
	case "z1", "huabei":
		return &storage.ZoneHuabei
	case "z2", "huanan":
		return &storage.ZoneHuanan
	case "na0", "beimei":
		return &storage.ZoneBeimei
	case "as0", "xinjiapo":
		return &storage.ZoneXinjiapo
	default:
		// 默认使用华东区域
		return &storage.ZoneHuadong
	}
}

// getUploadConfig 获取上传配置
func (p *QiniuKodoProvider) getUploadConfig() *storage.Config {
	cfg := storage.Config{
		Zone:                p.getRegion(),
		Region:              p.getRegion(),
		UseHTTPS:            true,
		UseCdnDomains:       false,
		AccelerateUploading: false,
		CentralRsHost:       storage.DefaultRsHost,
	}
	return &cfg
}

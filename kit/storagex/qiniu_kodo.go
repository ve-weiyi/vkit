package storagex

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
)

// QiniuKodoProvider 七牛云Kodo存储服务提供商
type QiniuKodoProvider struct {
	config *StorageConfig
	mac    *qbox.Mac
}

// NewQiniuKodoProvider 创建七牛云Kodo存储服务提供商实例
func NewQiniuKodoProvider(config *StorageConfig) *QiniuKodoProvider {
	// 创建认证对象
	mac := qbox.NewMac(config.AccessKey, config.SecretKey)

	return &QiniuKodoProvider{
		config: config,
		mac:    mac,
	}
}

// GetUploadToken 获取上传凭证
func (p *QiniuKodoProvider) GetUploadToken(ctx context.Context, filename string, expireSeconds int) (*UploadToken, error) {
	// 生成唯一文件Key
	fileKey := p.generateFileKey(filename)

	// 构建上传策略
	putPolicy := storage.PutPolicy{
		Scope:   fmt.Sprintf("%s:%s", p.config.Bucket, fileKey),
		Expires: uint64(expireSeconds),
	}

	// 生成上传凭证
	uploadToken := putPolicy.UploadToken(p.mac)

	// 获取上传区域
	region := p.getRegion()

	// 构建上传URL
	uploadURL := region.SrcUpHosts[0]
	if !strings.HasPrefix(uploadURL, "http") {
		uploadURL = "https://" + uploadURL
	}

	// 构建访问URL
	accessURL := p.buildAccessURL(fileKey)

	return &UploadToken{
		UploadURL: uploadURL,
		Token:     uploadToken,
		FileKey:   fileKey,
		AccessURL: accessURL,
		ExpireAt:  time.Now().Add(time.Duration(expireSeconds) * time.Second),
		ExtraData: map[string]string{
			"provider": "qiniu",
			"bucket":   p.config.Bucket,
		},
	}, nil
}

// Upload 服务端上传文件
func (p *QiniuKodoProvider) Upload(ctx context.Context, file io.Reader, filename string) (string, error) {
	// 生成唯一文件Key
	fileKey := p.generateFileKey(filename)

	// 构建上传策略
	putPolicy := storage.PutPolicy{
		Scope: fmt.Sprintf("%s:%s", p.config.Bucket, fileKey),
	}
	uploadToken := putPolicy.UploadToken(p.mac)

	// 获取上传配置
	cfg := p.getUploadConfig()

	// 创建表单上传对象
	formUploader := storage.NewFormUploader(cfg)
	ret := storage.PutRet{}
	putExtra := storage.PutExtra{}
	// 上传文件
	err := formUploader.Put(ctx, &ret, uploadToken, fileKey, file, -1, &putExtra)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// 返回访问URL
	return p.buildAccessURL(fileKey), nil
}

// Delete 删除文件
func (p *QiniuKodoProvider) Delete(ctx context.Context, fileURL string) error {
	// 从URL中提取文件Key
	fileKey := p.extractFileKey(fileURL)

	// 获取存储配置
	cfg := p.getUploadConfig()

	// 创建存储管理对象
	bucketManager := storage.NewBucketManager(p.mac, cfg)

	// 删除文件
	err := bucketManager.Delete(p.config.Bucket, fileKey)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// GetAccessURL 获取文件访问URL
func (p *QiniuKodoProvider) GetAccessURL(ctx context.Context, fileKey string, expireSeconds int) (string, error) {
	// 如果是私有存储，生成签名URL
	if p.config.IsPrivate {
		deadline := time.Now().Add(time.Duration(expireSeconds) * time.Second).Unix()
		privateURL := storage.MakePrivateURL(p.mac, p.config.CDNDomain, p.buildAccessURL(fileKey), deadline)
		return privateURL, nil
	}

	// 公共存储，直接返回访问URL
	return p.buildAccessURL(fileKey), nil
}

// ListFiles 列举文件
func (p *QiniuKodoProvider) ListFiles(ctx context.Context, prefix string, limit int) ([]*FileInfo, error) {
	cfg := p.getUploadConfig()
	bucketManager := storage.NewBucketManager(p.mac, cfg)

	entries, prefixes, _, _, err := bucketManager.ListFiles(p.config.Bucket, prefix, "/", "", limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	files := make([]*FileInfo, 0, len(prefixes)+len(entries))
	for _, fix := range prefixes {
		files = append(files, &FileInfo{
			IsDir:    true,
			FilePath: fix,
			FileName: filepath.Base(fix),
			FileURL:  p.buildAccessURL(fix),
		})
	}
	for _, entry := range entries {
		if entry.Fsize == 0 {
			continue
		}
		files = append(files, &FileInfo{
			IsDir:    false,
			FilePath: entry.Key,
			FileName: filepath.Base(entry.Key),
			FileType: filepath.Ext(entry.Key),
			FileSize: entry.Fsize,
			FileURL:  p.buildAccessURL(entry.Key),
			UpTime:   time.UnixMicro(entry.PutTime / 10),
		})
	}
	return files, nil
}

// GetProviderName 获取服务商名称
func (p *QiniuKodoProvider) GetProviderName() string {
	return "qiniu"
}

// generateFileKey 生成唯一的文件Key
func (p *QiniuKodoProvider) generateFileKey(filename string) string {
	return GenerateFileKey(p.config.BasePath, filename)
}

// buildAccessURL 构建访问URL
func (p *QiniuKodoProvider) buildAccessURL(fileKey string) string {
	// 如果配置了CDN域名，使用CDN域名
	if p.config.CDNDomain != "" {
		return buildURLWithDomain(p.config.CDNDomain, fileKey)
	}

	// 使用七牛云默认域名（需要在控制台配置）
	// 注意：七牛云必须配置CDN域名才能访问，这里返回一个占位符
	return fmt.Sprintf("https://%s.qiniucdn.com/%s", p.config.Bucket, fileKey)
}

// extractFileKey 从URL中提取文件Key
func (p *QiniuKodoProvider) extractFileKey(fileURL string) string {
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

// getRegion 获取存储区域配置
func (p *QiniuKodoProvider) getRegion() *storage.Zone {
	// 根据配置的Region返回对应的区域
	switch p.config.Region {
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

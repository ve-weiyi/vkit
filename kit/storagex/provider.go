package storagex

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

// StorageProvider 存储服务提供商接口
// 支持多种存储服务：阿里云OSS、腾讯云COS、七牛云、本地存储等
type StorageProvider interface {
	// GetUploadToken 获取上传凭证（用于前端直传）
	// filename: 文件名
	// expireSeconds: 凭证有效期（秒）
	// returns: 上传凭证信息
	GetUploadToken(ctx context.Context, filename string, expireSeconds int) (*UploadToken, error)

	// Upload 服务端上传文件
	// file: 文件内容
	// filename: 文件名
	// returns: 文件访问URL
	Upload(ctx context.Context, file io.Reader, filename string) (string, error)

	// Delete 删除文件
	// fileURL: 文件URL或文件Key
	Delete(ctx context.Context, fileURL string) error

	// GetAccessURL 获取文件访问URL（支持私有文件临时访问）
	// fileKey: 文件Key
	// expireSeconds: URL有效期（秒），0表示永久
	// returns: 访问URL
	GetAccessURL(ctx context.Context, fileKey string, expireSeconds int) (string, error)

	// ListFiles 列举文件
	// prefix: 文件前缀（目录路径）
	// limit: 最大返回数量
	ListFiles(ctx context.Context, prefix string, limit int) ([]*FileInfo, error)

	// GetProviderName 获取服务商名称
	GetProviderName() string
}

// FileInfo 文件信息
type FileInfo struct {
	IsDir    bool      `json:"is_dir"`
	FilePath string    `json:"file_path"`
	FileName string    `json:"file_name"`
	FileType string    `json:"file_type"`
	FileSize int64     `json:"file_size"`
	FileURL  string    `json:"file_url"`
	UpTime   time.Time `json:"up_time"`
}

// UploadToken 上传凭证信息（用于前端直传）
type UploadToken struct {
	UploadURL string            `json:"upload_url"` // 上传地址
	Token     string            `json:"token"`      // 上传凭证/Token
	Policy    string            `json:"policy"`     // 上传策略（部分服务商需要）
	Signature string            `json:"signature"`  // 签名（部分服务商需要）
	FileKey   string            `json:"file_key"`   // 文件Key/路径
	AccessURL string            `json:"access_url"` // 上传成功后的访问URL
	ExpireAt  time.Time         `json:"expire_at"`  // 凭证过期时间
	ExtraData map[string]string `json:"extra_data"` // 额外数据（服务商特定字段）
}

// StorageConfig 存储服务配置
type StorageConfig struct {
	Provider  string // 服务商类型：aliyun | tencent | qiniu | local
	Endpoint  string // 服务端点（OSS/COS endpoint）
	Bucket    string // 存储桶名称
	AccessKey string // AccessKey / SecretId
	SecretKey string // SecretKey
	Region    string // 地域
	CDNDomain string // CDN域名（可选）
	BasePath  string // 文件存储基础路径（可选）
	IsPrivate bool   // 是否私有存储
}

// NewStorageProvider 创建存储服务提供商实例（工厂模式）
func NewStorageProvider(config *StorageConfig) StorageProvider {
	switch config.Provider {
	case "aliyun":
		return NewAliyunOSSProvider(config)
	case "tencent":
		return NewTencentCOSProvider(config)
	case "qiniu":
		return NewQiniuKodoProvider(config)
	case "local":
		return NewLocalStorageProvider(config)
	default:
		// 默认使用本地存储（开发环境）
		return NewLocalStorageProvider(config)
	}
}

// buildURLWithDomain 构建带域名的URL，若域名已含协议则直接使用，否则默认补 https://
func buildURLWithDomain(domain, fileKey string) string {
	domain = strings.TrimRight(domain, "/")
	if !strings.HasPrefix(domain, "http://") && !strings.HasPrefix(domain, "https://") {
		domain = "https://" + domain
	}
	return fmt.Sprintf("%s/%s", domain, fileKey)
}

// GenerateFileKey 生成唯一的文件Key
func GenerateFileKey(basePath, filename string) string {
	// 获取文件目录
	dir := filepath.Dir(filename)
	// 获取文件名称
	base := filepath.Base(filename)

	// 生成日期路径：YYYYMMDD
	now := time.Now()
	datePath := now.Format("20060102")

	// 生成唯一文件名：毫秒时间戳 + 文件名称
	uniqueName := fmt.Sprintf("%d-%s", now.UnixMilli(), base)

	// 组合完整路径（包含BasePath）
	if basePath != "" {
		return filepath.Join(basePath, datePath, uniqueName)
	}

	return filepath.Join(dir, datePath, uniqueName)
}

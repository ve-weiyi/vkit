package storagex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

// ErrNotFound 对象不存在（Download/Stat/Delete 等按 key 操作时返回）。
var ErrNotFound = errors.New("storagex: object not found")

// StorageProvider 存储服务提供商接口
// 支持多种存储服务：阿里云OSS、腾讯云COS、七牛云、本地存储等
type StorageProvider interface {
	// Upload 服务端上传文件
	// file: 文件内容
	// name: 文件名
	// opts: 可选参数（fileKey、contentType 等）
	Upload(ctx context.Context, file io.Reader, name string, opts ...UploadOption) (*UploadResult, error)

	// Download 按 key 下载文件全量内容
	// fileKey: 文件Key
	// returns: 文件内容字节；对象不存在 → ErrNotFound
	Download(ctx context.Context, fileKey string) ([]byte, error)

	// UploadTokenResult 获取上传凭证（用于前端直传）
	// name: 文件名
	// expire: 凭证有效期
	// opts: 可选参数（fileKey 等），与 Upload 共享同一套 UploadOption
	UploadToken(ctx context.Context, name string, expire time.Duration, opts ...UploadOption) (*UploadTokenResult, error)

	// SignURL 生成文件签名访问URL（支持私有文件临时访问）
	// fileKey: 文件Key
	// expire: URL有效期，≤0 使用默认值（1小时）
	// returns: 访问URL
	SignURL(ctx context.Context, fileKey string, expire time.Duration) (string, error)

	// Stat 获取文件元信息
	// fileKey: 文件Key
	Stat(ctx context.Context, fileKey string) (*FileStat, error)

	// List 列举文件
	// prefix: 文件前缀（目录路径）
	// marker: 分页游标，空串表示从头开始
	// limit: 最大返回数量
	List(ctx context.Context, prefix string, marker string, limit int) (*ListResult, error)

	// Delete 删除文件
	// fileKey: 文件Key
	Delete(ctx context.Context, fileKey string) error

	// AccessURL 构建文件对外访问 URL（按服务商配置推导：CDN/Endpoint/Bucket 等）
	AccessURL(fileKey string) string

	// ProviderName 获取服务商名称
	ProviderName() string
}

// OSSConfig 阿里云 OSS 存储配置
type OSSConfig struct {
	Endpoint  string // 服务端点，如 oss-cn-hangzhou.aliyuncs.com
	Bucket    string // 存储桶名称
	AccessKey string // AccessKey
	SecretKey string // SecretKey
	IsPrivate bool   // 是否私有存储
	CDNDomain string `json:",optional"` // CDN 域名
}

// COSConfig 腾讯云 COS 存储配置
type COSConfig struct {
	Endpoint  string // 服务端点，如 https://mybucket.cos.ap-guangzhou.myqcloud.com
	Bucket    string // 存储桶名称
	SecretID  string // SecretId
	SecretKey string // SecretKey
	IsPrivate bool   `json:",optional"` // 是否私有存储
	CDNDomain string `json:",optional"` // CDN 域名
}

// MinioConfig MinIO 存储配置
type MinioConfig struct {
	Endpoint  string           // 服务端点，如 127.0.0.1:9000
	Bucket    string           // 存储桶名称
	AccessKey string           // AccessKey
	SecretKey string           // SecretKey
	Secure    bool             `json:",optional"` // 是否使用 TLS（k3s 集群内通信不需要）
	CDNDomain string           `json:",optional"` // Ingress 域名
	Lifecycle *LifecycleConfig `json:",optional"` // 生命周期配置
}

// KodoConfig 七牛云 Kodo 存储配置
type KodoConfig struct {
	Endpoint  string // 服务端点，如 https://s3.cn-south-1.qiniucs.com
	Bucket    string // 存储桶名称
	AccessKey string // AccessKey
	SecretKey string // SecretKey
	Region    string // 存储区域，如 huanan / huadong / huabei / beimei / xinjiapo
	IsPrivate bool   `json:",optional"` // 是否私有存储
	CDNDomain string `json:",optional"` // CDN 域名
}

// LocalConfig 本地存储配置
type LocalConfig struct {
	Endpoint  string `json:",optional"` // HTTP 端点
	CDNDomain string `json:",optional"` // CDN 域名
	Dir       string `json:",optional"` // 本地文件存储目录
}

// StorageConfig 存储服务配置容器
type StorageConfig struct {
	Provider string       // 服务商类型：aliyun | tencent | qiniu | local | minio
	OSS      *OSSConfig   `json:",optional"`
	COS      *COSConfig   `json:",optional"`
	Minio    *MinioConfig `json:",optional"`
	Kodo     *KodoConfig  `json:",optional"`
	Local    *LocalConfig `json:",optional"`
}

// LifecycleConfig 对象生命周期配置
type LifecycleConfig struct {
	ExpireDays int // 文件过期天数，0 表示不启用
}

// UploadResult 上传文件结果
type UploadResult struct {
	AccessURL string // 文件访问URL
	FileKey   string // 文件Key
}

// UploadTokenResult 上传凭证信息（用于前端直传）
type UploadTokenResult struct {
	UploadURL string            `json:"upload_url"` // 上传地址
	Token     string            `json:"token"`      // 上传凭证/Token
	Policy    string            `json:"policy"`     // 上传策略（部分服务商需要）
	Signature string            `json:"signature"`  // 签名（部分服务商需要）
	FileKey   string            `json:"file_key"`   // 文件Key/路径
	AccessURL string            `json:"access_url"` // 上传成功后的访问URL
	ExpireAt  time.Time         `json:"expire_at"`  // 凭证过期时间
	ExtraData map[string]string `json:"extra_data"` // 额外数据（服务商特定字段）
}

// ListResult 列举文件结果
type ListResult struct {
	Files      []*FileStat // 文件列表
	NextMarker string      // 下一页游标，空串表示已到末尾
}

// FileStat 文件信息
type FileStat struct {
	IsDir        bool      `json:"is_dir"`
	FileKey      string    `json:"file_key"`
	FileName     string    `json:"file_name"`
	FileSize     int64     `json:"file_size"`
	FileURL      string    `json:"file_url"`
	ContentType  string    `json:"content_type"` // MIME 类型（Stat 返回）
	LastModified time.Time `json:"last_modified"`
}

// UploadOptions 上传可选参数
type UploadOptions struct {
	FileKey     string // 预计算的文件 key，非空时跳过内部 generateFileKey
	ContentType string // MIME 类型（可选）
}

// UploadOption 上传选项函数
type UploadOption func(*UploadOptions)

// WithFileKey 指定预计算的文件 key
func WithFileKey(key string) UploadOption {
	return func(o *UploadOptions) { o.FileKey = key }
}

// WithContentType 指定文件 MIME 类型
func WithContentType(ct string) UploadOption {
	return func(o *UploadOptions) { o.ContentType = ct }
}

// resolveOpts 解析上传选项，填充 fileKey 和 contentType 的默认值
func resolveOpts(opts []UploadOption, fallbackKey string) *UploadOptions {
	var o UploadOptions
	for _, opt := range opts {
		opt(&o)
	}
	if o.FileKey == "" {
		o.FileKey = fallbackKey
	}
	if o.ContentType == "" {
		o.ContentType = "application/octet-stream"
	}
	return &o
}

// Option 存储 Provider 构造选项：声明 Provider 支持的行为与参数（NewStorageProvider 使用）。
// 行为按后端能力装配——minio 等支持生命周期的后端下发规则，local 等无能力后端忽略对应 option。
type Option func(*options)

type options struct {
	retentionDays int // 图像保留天数（0 = 不启用生命周期/清理）
}

// WithRetentionDays 声明保留期行为并配置天数（0 = 不启用）。
// minio 下发桶生命周期规则（对象自动过期）；local 记录保留期并支持 CleanExpired 扫目录自删。
func WithRetentionDays(days int) Option {
	return func(o *options) { o.retentionDays = days }
}

// NewStorageProvider 创建存储服务提供商实例（工厂模式）：cfg 选择后端，opts 声明启用的行为。
// 行为由各后端构造器按能力装配——minio 支持保留期生命周期，local 等无能力后端忽略对应 option。
// provider 名常量：这些字面量同时出现在工厂分支、ProviderName 与返回体里，
// 任意一处拼错都会静默走错分支（曾因此把未知 provider 放过）。
const (
	ProviderAliyun  = "aliyun"
	ProviderTencent = "tencent"
	ProviderQiniu   = "qiniu"
	ProviderMinio   = "minio"
	ProviderLocal   = "local"
)

func NewStorageProvider(cfg *StorageConfig, opts ...Option) (StorageProvider, error) {
	switch cfg.Provider {
	case ProviderAliyun:
		if cfg.OSS == nil {
			return nil, errors.New("storagex: provider is aliyun but OSS config is nil")
		}
		return NewAliyunOSSProvider(cfg.OSS, opts...)
	case ProviderTencent:
		if cfg.COS == nil {
			return nil, errors.New("storagex: provider is tencent but COS config is nil")
		}
		return NewTencentCOSProvider(cfg.COS, opts...), nil
	case ProviderQiniu:
		if cfg.Kodo == nil {
			return nil, errors.New("storagex: provider is qiniu but Kodo config is nil")
		}
		return NewQiniuKodoProvider(cfg.Kodo, opts...), nil
	case ProviderMinio:
		if cfg.Minio == nil {
			return nil, errors.New("storagex: provider is minio but Minio config is nil")
		}
		return NewMinioProvider(cfg.Minio, opts...)
	case ProviderLocal:
		if cfg.Local == nil {
			cfg.Local = &LocalConfig{}
		}
		return NewLocalStorageProvider(cfg.Local, opts...), nil
	default:
		return nil, fmt.Errorf("storagex: unsupported provider %q (want aliyun | tencent | qiniu | minio | local)", cfg.Provider)
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
// key 格式：{filename中的目录}/{YYYYMMDD}/{毫秒时间戳}-{basename}
func GenerateFileKey(filename string) string {
	// 获取文件目录
	dir := filepath.Dir(filename)
	// 获取文件名称
	base := filepath.Base(filename)

	// 生成日期路径：YYYYMMDD
	now := time.Now()
	datePath := now.Format("20060102")

	// 生成唯一文件名：毫秒时间戳 + 文件名称
	uniqueName := fmt.Sprintf("%d-%s", now.UnixMilli(), base)

	if dir != "." {
		return filepath.Join(dir, datePath, uniqueName)
	}
	return filepath.Join(datePath, uniqueName)
}

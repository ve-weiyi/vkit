package oauthx

import (
	"fmt"
	"net/url"
)

// provider 名常量：避免字面量在多处拼错导致走错分支
const (
	ProviderQQ     = "qq"
	ProviderGithub = "github"
	ProviderGitee  = "gitee"
	ProviderWeibo  = "weibo"
	ProviderFeishu = "feishu"
)

// NewOAuthProvider 创建OAuth服务提供商实例（工厂模式）。
// 未知平台返回 error 而不是 nil：返回 nil 接口会让调用方在首次使用时 panic。
func NewOAuthProvider(config *OAuthConfig) (OAuthProvider, error) {
	switch config.Platform {
	case ProviderQQ:
		return newQQProvider(config), nil
	case ProviderGithub:
		return newGithubProvider(config), nil
	case ProviderGitee:
		return newGiteeProvider(config), nil
	case ProviderWeibo:
		return newWeiboProvider(config), nil
	case ProviderFeishu:
		return newFeishuProvider(config), nil
	default:
		return nil, fmt.Errorf("oauthx: unsupported platform %q", config.Platform)
	}
}

// buildAuthorizeURL 为目标地址追加查询参数，生成授权页跳转链接。
func buildAuthorizeURL(base string, params map[string]string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base
	}

	query := u.Query()
	for key, value := range params {
		query.Set(key, value)
	}
	u.RawQuery = query.Encode()

	return u.String()
}

package oauthx

// NewOAuthProvider 创建OAuth服务提供商实例（工厂模式）
func NewOAuthProvider(config *OAuthConfig) OAuthProvider {
	switch config.Platform {
	case "qq":
		return newQQProvider(config)
	case "github":
		return newGithubProvider(config)
	case "gitee":
		return newGiteeProvider(config)
	case "weibo":
		return newWeiboProvider(config)
	case "feishu":
		return newFeishuProvider(config)
	default:
		return nil
	}
}

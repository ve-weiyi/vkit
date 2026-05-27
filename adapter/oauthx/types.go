package oauthx

// UserResult 统一的用户信息结构
type UserResult struct {
	OpenId   string `json:"open_id"`    // 用户在开放平台的唯一标识
	NickName string `json:"nick_name"`  // 用户昵称
	Name     string `json:"name"`       // 用户姓名
	EnName   string `json:"en_name"`    // 用户英文名
	Avatar   string `json:"avatar_url"` // 头像URL
	Email    string `json:"email"`      // 邮箱
	Mobile   string `json:"mobile"`     // 手机号码
}

// TokenResult 统一的Token结构
type TokenResult struct {
	AccessToken  string `json:"access_token"`  // 访问令牌
	RefreshToken string `json:"refresh_token"` // 刷新令牌
	ExpiresIn    int64  `json:"expires_in"`    // 过期时间（秒）
	TokenType    string `json:"token_type"`    // 令牌类型
	Scope        string `json:"scope"`         // 授权范围
	OpenId       string `json:"open_id"`       // 用户OpenID（部分平台需要）
	UnionId      string `json:"union_id"`      // 用户UnionID（部分平台需要）
}

// OAuthProvider OAuth服务提供商接口
type OAuthProvider interface {
	// GetName 获取平台名称
	GetName() string

	// GetAuthLoginUrl 获取授权登录URL
	GetAuthLoginUrl(state string) string

	// GetAuthUserInfo 通过授权码获取用户信息
	GetAuthUserInfo(code string) (*UserResult, error)
}

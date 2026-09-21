package oauthx

import (
	"context"
	"log"

	"github.com/ve-weiyi/vkit/x/httpx"
)

type qqProvider struct {
	config          *OAuthConfig
	client          *httpx.Client
	authorizeUrl    string
	accessTokenUrl  string
	refreshTokenUrl string
	openidUrl       string
	userInfoUrl     string
}

func newQQProvider(config *OAuthConfig) *qqProvider {
	return &qqProvider{
		config:          config,
		client:          httpx.New(),
		authorizeUrl:    "https://graph.qq.com/oauth2.0/authorize",
		accessTokenUrl:  "https://graph.qq.com/oauth2.0/token",
		refreshTokenUrl: "https://graph.qq.com/oauth2.0/token",
		openidUrl:       "https://graph.qq.com/oauth2.0/me",
		userInfoUrl:     "https://graph.qq.com/user/get_user_info",
	}
}

func (p *qqProvider) GetName() string { return ProviderQQ }

func (p *qqProvider) GetAuthLoginUrl(state string) string {
	return buildAuthorizeURL(p.authorizeUrl, map[string]string{
		"client_id":     p.config.ClientId,
		"redirect_uri":  p.config.RedirectUri,
		"state":         state,
		"response_type": "code",
	})
}

func (p *qqProvider) GetAuthUserInfo(ctx context.Context, code string) (*UserResult, error) {
	token, err := p.getAccessToken(ctx, code)
	if err != nil {
		return nil, err
	}

	open, err := p.getOpenid(ctx, token.AccessToken)
	if err != nil {
		return nil, err
	}

	user, err := p.getUserInfo(ctx, token.AccessToken, open.OpenId)
	if err != nil {
		return nil, err
	}

	return &UserResult{
		OpenId:   open.OpenId,
		NickName: user.Nickname,
		Name:     user.Nickname,
		EnName:   user.Nickname,
		Avatar:   user.FigureURLQQ1,
	}, nil
}

func (p *qqProvider) getAccessToken(ctx context.Context, code string) (*qqToken, error) {
	resp, err := p.client.Get(ctx, p.accessTokenUrl,
		httpx.WithQuery("client_id", p.config.ClientId),
		httpx.WithQuery("client_secret", p.config.ClientSecret),
		httpx.WithQuery("redirect_uri", p.config.RedirectUri),
		httpx.WithQuery("code", code),
		httpx.WithQuery("grant_type", "authorization_code"),
		httpx.WithQuery("fmt", "json"),
	)
	if err != nil {
		return nil, err
	}
	log.Println("qq token:", string(resp.Body))

	var out qqToken
	return &out, resp.Unmarshal(&out)
}

func (p *qqProvider) getOpenid(ctx context.Context, accessToken string) (*qqOpenResult, error) {
	resp, err := p.client.Get(ctx, p.openidUrl,
		httpx.WithQuery("access_token", accessToken),
		httpx.WithQuery("fmt", "json"),
	)
	if err != nil {
		return nil, err
	}
	log.Println("qq openid:", string(resp.Body))

	var out qqOpenResult
	return &out, resp.Unmarshal(&out)
}

func (p *qqProvider) getUserInfo(ctx context.Context, accessToken, openId string) (*qqUserInfo, error) {
	resp, err := p.client.Get(ctx, p.userInfoUrl,
		httpx.WithQuery("openid", openId),
		httpx.WithQuery("access_token", accessToken),
		httpx.WithQuery("oauth_consumer_key", p.config.ClientId),
	)
	if err != nil {
		return nil, err
	}
	log.Println("qq userinfo:", string(resp.Body))

	var out qqUserInfo
	return &out, resp.Unmarshal(&out)
}

type qqToken struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    string `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

type qqOpenResult struct {
	OpenId  string `json:"openId"`
	Unionid string `json:"unionid"`
}

type qqUserInfo struct {
	Ret          int    `json:"ret"`
	Msg          string `json:"msg"`
	Nickname     string `json:"nickname"`
	FigureURLQQ1 string `json:"figureurl_qq_1"`
	FigureURLQQ2 string `json:"figureurl_qq_2"`
}

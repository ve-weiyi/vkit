package oauthx

import (
	"context"
	"fmt"
	"log"

	"github.com/ve-weiyi/vkit/x/httpx"
)

type feishuProvider struct {
	config             *OAuthConfig
	client             *httpx.Client
	authorizeUrl       string
	appAccessTokenUrl  string
	userAccessTokenUrl string
	userInfoUrl        string
}

func newFeishuProvider(config *OAuthConfig) *feishuProvider {
	return &feishuProvider{
		config:             config,
		client:             httpx.New(),
		authorizeUrl:       "https://open.feishu.cn/open-apis/authen/v1/authorize",
		appAccessTokenUrl:  "https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal",
		userAccessTokenUrl: "https://open.feishu.cn/open-apis/authen/v1/oidc/access_token",
		userInfoUrl:        "https://open.feishu.cn/open-apis/authen/v1/user_info",
	}
}

func (p *feishuProvider) GetName() string { return ProviderFeishu }

func (p *feishuProvider) GetAuthLoginUrl(state string) string {
	return buildAuthorizeURL(p.authorizeUrl, map[string]string{
		"app_id":       p.config.ClientId,
		"redirect_uri": p.config.RedirectUri,
		"scope":        "contact:user.base:readonly",
		"state":        state,
	})
}

func (p *feishuProvider) GetAuthUserInfo(ctx context.Context, code string) (*UserResult, error) {
	token, err := p.getUserAccessToken(ctx, code)
	if err != nil {
		return nil, err
	}

	info, err := p.getUserInfo(ctx, token.Data.AccessToken)
	if err != nil {
		return nil, err
	}

	u := info.Data
	return &UserResult{
		OpenId:   u.OpenId,
		NickName: u.EnName,
		Name:     u.Name,
		EnName:   u.EnName,
		Avatar:   u.AvatarUrl,
		Email:    u.Email,
		Mobile:   u.Mobile,
	}, nil
}

func (p *feishuProvider) getAppAccessToken(ctx context.Context) (*feishuAppTokenResp, error) {
	resp, err := p.client.Post(ctx, p.appAccessTokenUrl,
		httpx.WithQuery("app_id", p.config.ClientId),
		httpx.WithQuery("app_secret", p.config.ClientSecret),
	)
	if err != nil {
		return nil, err
	}
	log.Println("feishu app token:", string(resp.Body))

	var out feishuAppTokenResp
	if err = resp.Unmarshal(&out); err != nil {
		return nil, err
	}
	if out.Code != 0 {
		return nil, fmt.Errorf("get app access token failed: %s", out.Msg)
	}
	return &out, nil
}

func (p *feishuProvider) getUserAccessToken(ctx context.Context, code string) (*feishuUserAccessTokenResp, error) {
	tt, err := p.getAppAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Post(ctx, p.userAccessTokenUrl,
		httpx.WithHeader("Authorization", fmt.Sprintf("Bearer %s", tt.AppAccessToken)),
		httpx.WithJSON(map[string]any{
			"grant_type": "authorization_code",
			"code":       code,
		}),
	)
	if err != nil {
		return nil, err
	}
	log.Println("feishu user token:", string(resp.Body))

	var out feishuUserAccessTokenResp
	if err = resp.Unmarshal(&out); err != nil {
		return nil, err
	}
	if out.Code != 0 {
		return nil, fmt.Errorf("get user access token failed: %s", out.Msg)
	}
	return &out, nil
}

func (p *feishuProvider) getUserInfo(ctx context.Context, accessToken string) (*feishuUserInfoResp, error) {
	resp, err := p.client.Get(ctx, p.userInfoUrl,
		httpx.WithHeader("Authorization", fmt.Sprintf("Bearer %s", accessToken)),
		httpx.WithHeader("Content-Type", "application/json; charset=utf-8"),
	)
	if err != nil {
		return nil, err
	}
	log.Println("feishu userinfo:", string(resp.Body))

	var out feishuUserInfoResp
	if err = resp.Unmarshal(&out); err != nil {
		return nil, err
	}
	if out.Code != 0 {
		return nil, fmt.Errorf("get user info failed: %s", out.Msg)
	}
	return &out, nil
}

type feishuAppTokenResp struct {
	Code           int    `json:"code"`
	Msg            string `json:"msg"`
	AppAccessToken string `json:"app_access_token"`
	Expire         int    `json:"expire"`
}

type feishuUserAccessTokenResp struct {
	Code int                   `json:"code"`
	Msg  string                `json:"msg"`
	Data feishuUserAccessToken `json:"data"`
}

type feishuUserAccessToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

type feishuUserInfoResp struct {
	Code int            `json:"code"`
	Msg  string         `json:"msg"`
	Data feishuUserInfo `json:"data"`
}

type feishuUserInfo struct {
	Name      string `json:"name"`
	EnName    string `json:"en_name"`
	AvatarUrl string `json:"avatar_url"`
	OpenId    string `json:"open_id"`
	UnionId   string `json:"union_id"`
	Email     string `json:"email"`
	Mobile    string `json:"mobile"`
}

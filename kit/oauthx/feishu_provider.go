package oauthx

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/ve-weiyi/pkg/utils/httpx"
)

type feishuProvider struct {
	config             *OAuthConfig
	authorizeUrl       string
	appAccessTokenUrl  string
	userAccessTokenUrl string
	userInfoUrl        string
}

func newFeishuProvider(config *OAuthConfig) *feishuProvider {
	return &feishuProvider{
		config:             config,
		authorizeUrl:       "https://open.feishu.cn/open-apis/authen/v1/authorize",
		appAccessTokenUrl:  "https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal",
		userAccessTokenUrl: "https://open.feishu.cn/open-apis/authen/v1/oidc/access_token",
		userInfoUrl:        "https://open.feishu.cn/open-apis/authen/v1/user_info",
	}
}

func (p *feishuProvider) GetName() string { return "feishu" }

func (p *feishuProvider) GetAuthLoginUrl(state string) string {
	return httpx.NewRequest("GET", p.authorizeUrl,
		httpx.WithParams(map[string]string{
			"app_id":       p.config.ClientId,
			"redirect_uri": p.config.RedirectUri,
			"scope":        "contact:user.base:readonly",
			"state":        state,
		}),
	).EncodeURL()
}

func (p *feishuProvider) GetAuthUserInfo(code string) (*UserResult, error) {
	token, err := p.getUserAccessToken(code)
	if err != nil {
		return nil, err
	}

	info, err := p.getUserInfo(token.Data.AccessToken)
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

func (p *feishuProvider) getAppAccessToken() (*feishuAppTokenResp, error) {
	body, err := httpx.NewRequest("POST", p.appAccessTokenUrl,
		httpx.WithParams(map[string]string{
			"app_id":     p.config.ClientId,
			"app_secret": p.config.ClientSecret,
		}),
	).Do()
	if err != nil {
		return nil, err
	}
	log.Println("feishu app token:", string(body))
	var resp feishuAppTokenResp
	if err = json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("get app access token failed: %s", resp.Msg)
	}
	return &resp, nil
}

func (p *feishuProvider) getUserAccessToken(code string) (*feishuUserAccessTokenResp, error) {
	tt, err := p.getAppAccessToken()
	if err != nil {
		return nil, err
	}

	body, err := httpx.NewRequest("POST", p.userAccessTokenUrl,
		httpx.WithHeaders(map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", tt.AppAccessToken),
			"Content-Type":  "application/json; charset=utf-8",
		}),
		httpx.WithBodyJson(map[string]any{
			"grant_type": "authorization_code",
			"code":       code,
		}),
	).Do()
	if err != nil {
		return nil, err
	}
	log.Println("feishu user token:", string(body))
	var resp feishuUserAccessTokenResp
	if err = json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("get user access token failed: %s", resp.Msg)
	}
	return &resp, nil
}

func (p *feishuProvider) getUserInfo(accessToken string) (*feishuUserInfoResp, error) {
	body, err := httpx.NewRequest("GET", p.userInfoUrl,
		httpx.WithHeaders(map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", accessToken),
			"Content-Type":  "application/json; charset=utf-8",
		}),
	).Do()
	if err != nil {
		return nil, err
	}
	log.Println("feishu userinfo:", string(body))
	var resp feishuUserInfoResp
	if err = json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("get user info failed: %s", resp.Msg)
	}
	return &resp, nil
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

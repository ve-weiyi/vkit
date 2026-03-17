package oauthx

import (
	"encoding/json"
	"log"

	"github.com/ve-weiyi/pkg/utils/httpx"
)

type qqProvider struct {
	config          *OAuthConfig
	authorizeUrl    string
	accessTokenUrl  string
	refreshTokenUrl string
	openidUrl       string
	userInfoUrl     string
}

func newQQProvider(config *OAuthConfig) *qqProvider {
	return &qqProvider{
		config:          config,
		authorizeUrl:    "https://graph.qq.com/oauth2.0/authorize",
		accessTokenUrl:  "https://graph.qq.com/oauth2.0/token",
		refreshTokenUrl: "https://graph.qq.com/oauth2.0/token",
		openidUrl:       "https://graph.qq.com/oauth2.0/me",
		userInfoUrl:     "https://graph.qq.com/user/get_user_info",
	}
}

func (p *qqProvider) GetName() string { return "qq" }

func (p *qqProvider) GetAuthLoginUrl(state string) string {
	return httpx.NewRequest("GET", p.authorizeUrl,
		httpx.WithParams(map[string]string{
			"client_id":     p.config.ClientId,
			"redirect_uri":  p.config.RedirectUri,
			"state":         state,
			"response_type": "code",
		}),
	).EncodeURL()
}

func (p *qqProvider) GetAuthUserInfo(code string) (*UserResult, error) {
	token, err := p.getAccessToken(code)
	if err != nil {
		return nil, err
	}

	open, err := p.getOpenid(token.AccessToken)
	if err != nil {
		return nil, err
	}

	user, err := p.getUserInfo(token.AccessToken, open.OpenId)
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

func (p *qqProvider) getAccessToken(code string) (*qqToken, error) {
	body, err := httpx.NewRequest("GET", p.accessTokenUrl,
		httpx.WithParams(map[string]string{
			"client_id":     p.config.ClientId,
			"client_secret": p.config.ClientSecret,
			"redirect_uri":  p.config.RedirectUri,
			"code":          code,
			"grant_type":    "authorization_code",
			"fmt":           "json",
		}),
	).Do()
	if err != nil {
		return nil, err
	}
	log.Println("qq token:", string(body))
	var resp qqToken
	return &resp, json.Unmarshal(body, &resp)
}

func (p *qqProvider) getOpenid(accessToken string) (*qqOpenResult, error) {
	body, err := httpx.NewRequest("GET", p.openidUrl,
		httpx.WithParams(map[string]string{
			"access_token": accessToken,
			"fmt":          "json",
		}),
	).Do()
	if err != nil {
		return nil, err
	}
	log.Println("qq openid:", string(body))
	var resp qqOpenResult
	return &resp, json.Unmarshal(body, &resp)
}

func (p *qqProvider) getUserInfo(accessToken, openId string) (*qqUserInfo, error) {
	body, err := httpx.NewRequest("GET", p.userInfoUrl,
		httpx.WithParams(map[string]string{
			"openid":             openId,
			"access_token":       accessToken,
			"oauth_consumer_key": p.config.ClientId,
		}),
	).Do()
	if err != nil {
		return nil, err
	}
	log.Println("qq userinfo:", string(body))
	var resp qqUserInfo
	return &resp, json.Unmarshal(body, &resp)
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

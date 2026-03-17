package oauthx

import (
	"encoding/json"
	"log"
	"strconv"

	"github.com/ve-weiyi/pkg/utils/httpx"
)

type weiboProvider struct {
	config         *OAuthConfig
	authorizeUrl   string
	accessTokenUrl string
	userInfoUrl    string
}

func newWeiboProvider(config *OAuthConfig) *weiboProvider {
	return &weiboProvider{
		config:         config,
		authorizeUrl:   "https://api.weibo.com/oauth2/authorize",
		accessTokenUrl: "https://api.weibo.com/oauth2/access_token",
		userInfoUrl:    "https://api.weibo.com/2/users/show.json",
	}
}

func (p *weiboProvider) GetName() string { return "weibo" }

func (p *weiboProvider) GetAuthLoginUrl(state string) string {
	return httpx.NewRequest("GET", p.authorizeUrl,
		httpx.WithParams(map[string]string{
			"client_id":     p.config.ClientId,
			"redirect_uri":  p.config.RedirectUri,
			"state":         state,
			"response_type": "code",
		}),
	).EncodeURL()
}

func (p *weiboProvider) GetAuthUserInfo(code string) (*UserResult, error) {
	tk, err := p.getAccessToken(code)
	if err != nil {
		return nil, err
	}

	user, err := p.getUserInfo(tk.AccessToken, tk.Uid)
	if err != nil {
		return nil, err
	}

	return &UserResult{
		OpenId:   strconv.FormatInt(user.Id, 10),
		NickName: user.ScreenName,
		Name:     user.Name,
		Avatar:   user.AvatarLarge,
	}, nil
}

func (p *weiboProvider) getAccessToken(code string) (*weiboToken, error) {
	body, err := httpx.NewRequest("POST", p.accessTokenUrl,
		httpx.WithParams(map[string]string{
			"client_id":     p.config.ClientId,
			"client_secret": p.config.ClientSecret,
			"redirect_uri":  p.config.RedirectUri,
			"code":          code,
			"grant_type":    "authorization_code",
		}),
	).Do()
	if err != nil {
		return nil, err
	}
	log.Println("weibo token:", string(body))
	var resp weiboToken
	return &resp, json.Unmarshal(body, &resp)
}

func (p *weiboProvider) getUserInfo(accessToken, uid string) (*weiboUserInfo, error) {
	body, err := httpx.NewRequest("GET", p.userInfoUrl,
		httpx.WithParams(map[string]string{
			"uid":          uid,
			"access_token": accessToken,
		}),
	).Do()
	if err != nil {
		return nil, err
	}
	log.Println("weibo userinfo:", string(body))
	var resp weiboUserInfo
	return &resp, json.Unmarshal(body, &resp)
}

type weiboToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Uid         string `json:"uid"`
}

type weiboUserInfo struct {
	Id          int64  `json:"id"`
	ScreenName  string `json:"screen_name"`
	Name        string `json:"name"`
	AvatarLarge string `json:"avatar_large"`
}

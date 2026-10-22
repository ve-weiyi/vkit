package oauthx

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ve-weiyi/vkit/x/httpx"
)

type weiboProvider struct {
	config         *OAuthConfig
	client         *httpx.Client
	authorizeUrl   string
	accessTokenUrl string
	userInfoUrl    string
}

func newWeiboProvider(config *OAuthConfig) *weiboProvider {
	return &weiboProvider{
		config:         config,
		client:         httpx.New(),
		authorizeUrl:   "https://api.weibo.com/oauth2/authorize",
		accessTokenUrl: "https://api.weibo.com/oauth2/access_token",
		userInfoUrl:    "https://api.weibo.com/2/users/show.json",
	}
}

func (p *weiboProvider) GetName() string { return ProviderWeibo }

func (p *weiboProvider) GetAuthLoginUrl(state string) string {
	return buildAuthorizeURL(p.authorizeUrl, map[string]string{
		"client_id":     p.config.ClientId,
		"redirect_uri":  p.config.RedirectUri,
		"state":         state,
		"response_type": "code",
	})
}

func (p *weiboProvider) GetAuthUserInfo(ctx context.Context, code string) (*UserResult, error) {
	tk, err := p.getAccessToken(ctx, code)
	if err != nil {
		return nil, err
	}

	user, err := p.getUserInfo(ctx, tk.AccessToken, tk.Uid)
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

func (p *weiboProvider) getAccessToken(ctx context.Context, code string) (*weiboToken, error) {
	resp, err := p.client.Post(ctx, p.accessTokenUrl,
		httpx.WithForm(map[string]string{
			"client_id":     p.config.ClientId,
			"client_secret": p.config.ClientSecret,
			"redirect_uri":  p.config.RedirectUri,
			"code":          code,
			"grant_type":    "authorization_code",
		}),
	)
	if err != nil {
		return nil, err
	}

	var out weiboToken
	if err := resp.Unmarshal(&out); err != nil {
		return nil, err
	}
	if out.AccessToken == "" {
		return nil, fmt.Errorf("weibo oauth: token exchange failed: %s", strings.TrimSpace(string(resp.Body)))
	}
	return &out, nil
}

func (p *weiboProvider) getUserInfo(ctx context.Context, accessToken, uid string) (*weiboUserInfo, error) {
	resp, err := p.client.Get(ctx, p.userInfoUrl,
		httpx.WithQuery("uid", uid),
		httpx.WithQuery("access_token", accessToken),
	)
	if err != nil {
		return nil, err
	}

	var out weiboUserInfo
	return &out, resp.Unmarshal(&out)
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

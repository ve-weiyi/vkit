package oauthx

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ve-weiyi/vkit/x/httpx"
)

type giteeProvider struct {
	config         *OAuthConfig
	client         *httpx.Client
	authorizeUrl   string
	accessTokenUrl string
	userInfoUrl    string
}

func newGiteeProvider(config *OAuthConfig) *giteeProvider {
	return &giteeProvider{
		config:         config,
		client:         httpx.New(),
		authorizeUrl:   "https://gitee.com/oauth/authorize",
		accessTokenUrl: "https://gitee.com/oauth/token",
		userInfoUrl:    "https://gitee.com/api/v5/user",
	}
}

func (p *giteeProvider) GetName() string { return ProviderGitee }

func (p *giteeProvider) GetAuthLoginUrl(state string) string {
	return buildAuthorizeURL(p.authorizeUrl, map[string]string{
		"client_id":     p.config.ClientId,
		"redirect_uri":  p.config.RedirectUri,
		"state":         state,
		"response_type": "code",
	})
}

func (p *giteeProvider) GetAuthUserInfo(ctx context.Context, code string) (*UserResult, error) {
	token, err := p.getAccessToken(ctx, code)
	if err != nil {
		return nil, err
	}

	user, err := p.getUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, err
	}

	resp := &UserResult{
		OpenId:   strconv.Itoa(user.Id),
		NickName: user.Name,
		Name:     user.Login,
		EnName:   user.Login,
		Avatar:   user.AvatarUrl,
	}
	if user.Email != nil {
		resp.Email = *user.Email
	}
	return resp, nil
}

func (p *giteeProvider) getAccessToken(ctx context.Context, code string) (*giteeToken, error) {
	resp, err := p.client.Post(ctx, p.accessTokenUrl,
		httpx.WithHeader("Accept", "application/json"),
		httpx.WithForm(map[string]string{
			"client_id":     p.config.ClientId,
			"client_secret": p.config.ClientSecret,
			"code":          code,
			"redirect_uri":  p.config.RedirectUri,
			"grant_type":    "authorization_code",
		}),
	)
	if err != nil {
		return nil, err
	}

	var out giteeToken
	if err := resp.Unmarshal(&out); err != nil {
		return nil, err
	}
	if out.AccessToken == "" {
		return nil, fmt.Errorf("gitee oauth: token exchange failed: %s", strings.TrimSpace(string(resp.Body)))
	}
	return &out, nil
}

func (p *giteeProvider) getUserInfo(ctx context.Context, accessToken string) (*giteeUserInfo, error) {
	resp, err := p.client.Get(ctx, p.userInfoUrl,
		httpx.WithHeader("Authorization", fmt.Sprintf("Bearer %s", accessToken)),
		httpx.WithHeader("Content-Type", "application/json; charset=utf-8"),
	)
	if err != nil {
		return nil, err
	}

	var out giteeUserInfo
	return &out, resp.Unmarshal(&out)
}

type giteeToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	CreatedAt    int    `json:"created_at"`
}

type giteeUserInfo struct {
	Id        int       `json:"id"`
	Login     string    `json:"login"`
	Name      string    `json:"name"`
	AvatarUrl string    `json:"avatar_url"`
	Email     *string   `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

package oauthx

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ve-weiyi/vkit/x/httpx"
)

type githubProvider struct {
	config         *OAuthConfig
	client         *httpx.Client
	authorizeUrl   string
	accessTokenUrl string
	userInfoUrl    string
}

func newGithubProvider(config *OAuthConfig) *githubProvider {
	return &githubProvider{
		config:         config,
		client:         httpx.New(),
		authorizeUrl:   "https://github.com/login/oauth/authorize",
		accessTokenUrl: "https://github.com/login/oauth/access_token",
		userInfoUrl:    "https://api.github.com/user",
	}
}

func (p *githubProvider) GetName() string { return ProviderGithub }

func (p *githubProvider) GetAuthLoginUrl(state string) string {
	return buildAuthorizeURL(p.authorizeUrl, map[string]string{
		"client_id":    p.config.ClientId,
		"redirect_uri": p.config.RedirectUri,
		"state":        state,
	})
}

func (p *githubProvider) GetAuthUserInfo(ctx context.Context, code string) (*UserResult, error) {
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

func (p *githubProvider) getAccessToken(ctx context.Context, code string) (*githubToken, error) {
	resp, err := p.client.Post(ctx, p.accessTokenUrl,
		httpx.WithHeader("Authorization", fmt.Sprintf("Bearer %s", code)),
		httpx.WithHeader("Content-Type", "application/json; charset=utf-8"),
		httpx.WithHeader("Accept", "application/json"),
		httpx.WithQuery("client_id", p.config.ClientId),
		httpx.WithQuery("client_secret", p.config.ClientSecret),
		httpx.WithQuery("code", code),
		httpx.WithQuery("redirect_uri", p.config.RedirectUri),
	)
	if err != nil {
		return nil, err
	}

	var out githubToken
	return &out, resp.Unmarshal(&out)
}

func (p *githubProvider) getUserInfo(ctx context.Context, accessToken string) (*githubUserInfo, error) {
	resp, err := p.client.Get(ctx, p.userInfoUrl,
		httpx.WithHeader("Authorization", fmt.Sprintf("Bearer %s", accessToken)),
		httpx.WithHeader("Content-Type", "application/json; charset=utf-8"),
	)
	if err != nil {
		return nil, err
	}

	var out githubUserInfo
	return &out, resp.Unmarshal(&out)
}

type githubToken struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}

type githubUserInfo struct {
	Login     string    `json:"login"`
	Id        int       `json:"id"`
	AvatarUrl string    `json:"avatar_url"`
	Name      string    `json:"name"`
	Email     *string   `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

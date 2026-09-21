package oauthx

import (
	"context"
	"fmt"
	"log"
	"strconv"
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
		httpx.WithHeader("Authorization", fmt.Sprintf("Bearer %s", code)),
		httpx.WithHeader("Content-Type", "application/json; charset=utf-8"),
		httpx.WithHeader("Accept", "application/json"),
		httpx.WithQuery("client_id", p.config.ClientId),
		httpx.WithQuery("client_secret", p.config.ClientSecret),
		httpx.WithQuery("code", code),
		httpx.WithQuery("redirect_uri", p.config.RedirectUri),
		httpx.WithQuery("grant_type", "authorization_code"),
	)
	if err != nil {
		return nil, err
	}
	log.Println("gitee token:", string(resp.Body))

	var out giteeToken
	return &out, resp.Unmarshal(&out)
}

func (p *giteeProvider) getUserInfo(ctx context.Context, accessToken string) (*giteeUserInfo, error) {
	resp, err := p.client.Get(ctx, p.userInfoUrl,
		httpx.WithHeader("Authorization", fmt.Sprintf("Bearer %s", accessToken)),
		httpx.WithHeader("Content-Type", "application/json; charset=utf-8"),
	)
	if err != nil {
		return nil, err
	}
	log.Println("gitee userinfo:", string(resp.Body))

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

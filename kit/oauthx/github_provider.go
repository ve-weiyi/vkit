package oauthx

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ve-weiyi/pkg/utils/httpx"
)

type githubProvider struct {
	config         *OAuthConfig
	authorizeUrl   string
	accessTokenUrl string
	userInfoUrl    string
}

func newGithubProvider(config *OAuthConfig) *githubProvider {
	return &githubProvider{
		config:         config,
		authorizeUrl:   "https://github.com/login/oauth/authorize",
		accessTokenUrl: "https://github.com/login/oauth/access_token",
		userInfoUrl:    "https://api.github.com/user",
	}
}

func (p *githubProvider) GetName() string { return "github" }

func (p *githubProvider) GetAuthLoginUrl(state string) string {
	return httpx.NewRequest("GET", p.authorizeUrl,
		httpx.WithParams(map[string]string{
			"client_id":    p.config.ClientId,
			"redirect_uri": p.config.RedirectUri,
			"state":        state,
		}),
	).EncodeURL()
}

func (p *githubProvider) GetAuthUserInfo(code string) (*UserResult, error) {
	token, err := p.getAccessToken(code)
	if err != nil {
		return nil, err
	}

	user, err := p.getUserInfo(token.AccessToken)
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

func (p *githubProvider) getAccessToken(code string) (*githubToken, error) {
	body, err := httpx.NewRequest("POST", p.accessTokenUrl,
		httpx.WithHeaders(map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", code),
			"Content-Type":  "application/json; charset=utf-8",
			"Accept":        "application/json",
		}),
		httpx.WithParams(map[string]string{
			"client_id":     p.config.ClientId,
			"client_secret": p.config.ClientSecret,
			"code":          code,
			"redirect_uri":  p.config.RedirectUri,
		}),
	).Do()
	if err != nil {
		return nil, err
	}
	var resp githubToken
	return &resp, json.Unmarshal(body, &resp)
}

func (p *githubProvider) getUserInfo(accessToken string) (*githubUserInfo, error) {
	body, err := httpx.NewRequest("GET", p.userInfoUrl,
		httpx.WithHeaders(map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", accessToken),
			"Content-Type":  "application/json; charset=utf-8",
		}),
	).Do()
	if err != nil {
		return nil, err
	}
	var resp githubUserInfo
	return &resp, json.Unmarshal(body, &resp)
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

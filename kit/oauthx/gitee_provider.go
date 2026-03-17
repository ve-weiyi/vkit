package oauthx

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/ve-weiyi/pkg/utils/httpx"
)

type giteeProvider struct {
	config         *OAuthConfig
	authorizeUrl   string
	accessTokenUrl string
	userInfoUrl    string
}

func newGiteeProvider(config *OAuthConfig) *giteeProvider {
	return &giteeProvider{
		config:         config,
		authorizeUrl:   "https://gitee.com/oauth/authorize",
		accessTokenUrl: "https://gitee.com/oauth/token",
		userInfoUrl:    "https://gitee.com/api/v5/user",
	}
}

func (p *giteeProvider) GetName() string { return "gitee" }

func (p *giteeProvider) GetAuthLoginUrl(state string) string {
	return httpx.NewRequest("GET", p.authorizeUrl,
		httpx.WithParams(map[string]string{
			"client_id":     p.config.ClientId,
			"redirect_uri":  p.config.RedirectUri,
			"state":         state,
			"response_type": "code",
		}),
	).EncodeURL()
}

func (p *giteeProvider) GetAuthUserInfo(code string) (*UserResult, error) {
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

func (p *giteeProvider) getAccessToken(code string) (*giteeToken, error) {
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
			"grant_type":    "authorization_code",
		}),
	).Do()
	if err != nil {
		return nil, err
	}
	log.Println("gitee token:", string(body))
	var resp giteeToken
	return &resp, json.Unmarshal(body, &resp)
}

func (p *giteeProvider) getUserInfo(accessToken string) (*giteeUserInfo, error) {
	body, err := httpx.NewRequest("GET", p.userInfoUrl,
		httpx.WithHeaders(map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", accessToken),
			"Content-Type":  "application/json; charset=utf-8",
		}),
	).Do()
	if err != nil {
		return nil, err
	}
	log.Println("gitee userinfo:", string(body))
	var resp giteeUserInfo
	return &resp, json.Unmarshal(body, &resp)
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

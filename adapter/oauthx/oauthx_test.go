package oauthx

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ve-weiyi/vkit/x/httpx"
)

type capturedRequest struct {
	method string
	query  url.Values
	form   url.Values
	json   map[string]any
}

func newTestConfig() *OAuthConfig {
	return &OAuthConfig{
		Platform:     "test",
		ClientId:     "client-id",
		ClientSecret: "client-secret",
		RedirectUri:  "https://example.com/callback",
	}
}

func newCaptureServer(t *testing.T, responseBody string) (*httptest.Server, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.query = r.URL.Query()

		if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &captured.json)
		} else {
			_ = r.ParseForm()
			captured.form = r.PostForm
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(responseBody))
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

func assertSecretOnlyInBody(t *testing.T, captured *capturedRequest) {
	t.Helper()

	if captured.method != http.MethodPost {
		t.Errorf("method = %s, want POST", captured.method)
	}
	if got := captured.query.Get("client_secret"); got != "" {
		t.Errorf("client_secret 出现在 query 中: %q", got)
	}
	if got := captured.query.Get("code"); got != "" {
		t.Errorf("authorization code 出现在 query 中: %q", got)
	}
	if got := captured.form.Get("client_secret"); got != "client-secret" {
		t.Errorf("client_secret = %q, want body value", got)
	}
}

func TestTokenExchangeKeepsSecretInBody(t *testing.T) {
	ctx := context.Background()
	cfg := newTestConfig()

	t.Run("github", func(t *testing.T) {
		srv, captured := newCaptureServer(t, `{"access_token":"token"}`)
		p := &githubProvider{config: cfg, client: httpx.New(), accessTokenUrl: srv.URL}

		if _, err := p.getAccessToken(ctx, "auth-code"); err != nil {
			t.Fatalf("getAccessToken: %v", err)
		}
		assertSecretOnlyInBody(t, captured)
	})

	t.Run("gitee", func(t *testing.T) {
		srv, captured := newCaptureServer(t, `{"access_token":"token"}`)
		p := &giteeProvider{config: cfg, client: httpx.New(), accessTokenUrl: srv.URL}

		if _, err := p.getAccessToken(ctx, "auth-code"); err != nil {
			t.Fatalf("getAccessToken: %v", err)
		}
		assertSecretOnlyInBody(t, captured)
	})

	t.Run("weibo", func(t *testing.T) {
		srv, captured := newCaptureServer(t, `{"access_token":"token","uid":"1"}`)
		p := &weiboProvider{config: cfg, client: httpx.New(), accessTokenUrl: srv.URL}

		if _, err := p.getAccessToken(ctx, "auth-code"); err != nil {
			t.Fatalf("getAccessToken: %v", err)
		}
		assertSecretOnlyInBody(t, captured)
	})

	t.Run("qq", func(t *testing.T) {
		srv, captured := newCaptureServer(t, `{"access_token":"token"}`)
		p := &qqProvider{config: cfg, client: httpx.New(), accessTokenUrl: srv.URL}

		if _, err := p.getAccessToken(ctx, "auth-code"); err != nil {
			t.Fatalf("getAccessToken: %v", err)
		}
		assertSecretOnlyInBody(t, captured)
	})
}

func TestFeishuAppTokenKeepsSecretInBody(t *testing.T) {
	srv, captured := newCaptureServer(t, `{"code":0,"app_access_token":"token"}`)
	p := &feishuProvider{config: newTestConfig(), client: httpx.New(), appAccessTokenUrl: srv.URL}

	if _, err := p.getAppAccessToken(context.Background()); err != nil {
		t.Fatalf("getAppAccessToken: %v", err)
	}

	if captured.method != http.MethodPost {
		t.Errorf("method = %s, want POST", captured.method)
	}
	if got := captured.query.Get("app_secret"); got != "" {
		t.Errorf("app_secret 出现在 query 中: %q", got)
	}
	if got, _ := captured.json["app_secret"].(string); got != "client-secret" {
		t.Errorf("app_secret = %q, want JSON body value", got)
	}
}

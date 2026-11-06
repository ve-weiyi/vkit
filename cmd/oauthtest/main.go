// 诊断程序：走真实 OAuth 授权码流程，验证 oauthx 的授权 URL 与用户信息交换。
//
// 需要真实 OAuth 应用凭据，故不放测试目录，由人工手动执行，非正式产物。
//
//	OAUTH_PLATFORM=github \
//	OAUTH_CLIENT_ID=... \
//	OAUTH_CLIENT_SECRET=... \
//	OAUTH_REDIRECT_URI=... \
//	go run ./cmd/oauthtest -mode=url
//
// 在浏览器完成授权后，把回调 URL 里的 code 传给 -code：
//
//	go run ./cmd/oauthtest -mode=exchange -code=...
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ve-weiyi/vkit/adapter/oauthx"
	"github.com/ve-weiyi/vkit/x/randomx"
	"github.com/ve-weiyi/vkit/x/redactx"
)

func main() {
	mode := flag.String("mode", "url", "url：生成授权 URL；exchange：用 code 换用户信息")
	code := flag.String("code", "", "回调 URL 中的 code（exchange 模式必填）")
	state := flag.String("state", "", "授权 state；为空时随机生成")
	flag.Parse()

	platform := requireEnv("OAUTH_PLATFORM")
	clientID := requireEnv("OAUTH_CLIENT_ID")
	clientSecret := requireEnv("OAUTH_CLIENT_SECRET")
	redirectURI := requireEnv("OAUTH_REDIRECT_URI")

	provider, err := oauthx.NewOAuthProvider(&oauthx.OAuthConfig{
		Platform:     platform,
		ClientId:     clientID,
		ClientSecret: clientSecret,
		RedirectUri:  redirectURI,
	})
	if err != nil {
		fatalf("NewOAuthProvider: %v", err)
	}

	switch *mode {
	case "url":
		if *state == "" {
			*state = randomx.GenerateRandomUUID()
		}
		fmt.Println(provider.GetAuthLoginUrl(*state))
	case "exchange":
		if strings.TrimSpace(*code) == "" {
			fatalf("exchange 模式必须通过 -code 提供回调 URL 中的 code")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		user, err := provider.GetAuthUserInfo(ctx, *code)
		if err != nil {
			fatalf("GetAuthUserInfo: %v", err)
		}

		fmt.Printf("platform=%s\nopen_id=%s\nname=%s\nnick_name=%s\navatar=%s\n",
			platform, user.OpenId, user.Name, user.NickName, user.Avatar)
		if user.Email != "" {
			fmt.Printf("email=%s\n", redactx.MaskEmail(user.Email))
		}
		if user.Mobile != "" {
			fmt.Printf("mobile=%s\n", redactx.MaskPhone(user.Mobile))
		}
	default:
		fatalf("未知 mode %q，支持 url | exchange", *mode)
	}
}

func requireEnv(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" || value == "xxx" {
		fatalf("缺少环境变量 %s（或仍是占位值 xxx）", name)
	}
	return value
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// 诊断程序：走真实短信通道发一条验证码，验证 adapter/smsx 的云厂商配置。
// 需要真实凭证且会真实发短信产生费用，故不放测试目录，由人工手动执行，非正式产物。
//
//	SMS_ACCESS_KEY=xx SMS_SECRET_KEY=xx \
//	  go run ./cmd/smstest -provider=aliyun -phone=13xxxxxxxxx -sign-name=签名 -template-code=SMS_xxx
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/ve-weiyi/vkit/adapter/smsx"
)

func main() {
	provider := flag.String("provider", "aliyun", "服务商：aliyun | tencent")
	phone := flag.String("phone", "", "接收手机号（必填）")
	signName := flag.String("sign-name", "", "短信签名")
	templateCode := flag.String("template-code", "", "验证码模板代码")
	region := flag.String("region", "ap-guangzhou", "地域（腾讯云需要）")
	sdkAppId := flag.String("sdk-app-id", "", "SDK 应用 ID（腾讯云需要）")
	code := flag.String("code", "123456", "验证码内容")
	expire := flag.Int("expire", 15, "过期时间（分钟）")
	flag.Parse()

	if *phone == "" {
		fatalf("必须指定 -phone")
	}
	accessKey := os.Getenv("SMS_ACCESS_KEY")
	secretKey := os.Getenv("SMS_SECRET_KEY")
	if accessKey == "" || secretKey == "" {
		fatalf("请通过环境变量 SMS_ACCESS_KEY / SMS_SECRET_KEY 提供凭证")
	}

	conf := &smsx.SmsConfig{
		Provider:  *provider,
		AccessKey: accessKey,
		SecretKey: secretKey,
		SignName:  *signName,
		Region:    *region,
		SdkAppId:  *sdkAppId,
	}
	if *templateCode != "" {
		conf.Templates = map[string]string{"login": *templateCode}
	}

	p, err := smsx.NewSmsProvider(conf)
	if err != nil {
		fatalf("NewSmsProvider: %v", err)
	}
	err = p.SendCode(context.Background(), *phone, "login", *code, *expire)
	if err != nil {
		fatalf("SendCode: %v", err)
	}

	fmt.Printf("发送成功: provider=%s phone=%s\n", p.GetProviderName(), *phone)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

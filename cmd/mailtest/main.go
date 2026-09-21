// 诊断程序：走真实 SMTP 投递一封邮件，验证 adapter/mail 的配置与连通性。
// 需要真实邮箱授权码，故不放测试目录，由人工手动执行，非正式产物。
//
//	MAIL_PASSWORD=授权码 go run ./cmd/mailtest -from=you@qq.com -to=someone@qq.com
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ve-weiyi/vkit/adapter/emailx"
)

func main() {
	host := flag.String("host", "smtp.qq.com", "SMTP 服务器地址")
	port := flag.Int("port", 465, "SMTP 端口")
	from := flag.String("from", "", "发件人邮箱（必填）")
	nickname := flag.String("nickname", "", "发件人昵称")
	to := flag.String("to", "", "收件人邮箱，多个以英文逗号分隔（必填）")
	cc := flag.String("cc", "", "抄送邮箱，多个以英文逗号分隔")
	bcc := flag.String("bcc", "", "密送邮箱，多个以英文逗号分隔")
	ssl := flag.Bool("ssl", true, "是否使用 SSL")
	flag.Parse()

	password := os.Getenv("MAIL_PASSWORD")
	if password == "" {
		fatalf("请通过环境变量 MAIL_PASSWORD 提供 SMTP 授权码")
	}
	if *from == "" || *to == "" {
		fatalf("必须指定 -from 与 -to")
	}

	deliver := emailx.NewEmailDeliver(&emailx.EmailConfig{
		Host:     *host,
		Port:     *port,
		Username: *from,
		Password: password,
		Nickname: *nickname,
		SSL:      *ssl,
		BCC:      splitList(*bcc),
	})

	err := deliver.DeliveryEmail(&emailx.EmailMessage{
		To:      splitList(*to),
		CC:      splitList(*cc),
		Subject: "vkit mailtest",
		Content: "这是一封来自 vkit/cmd/mailtest 的诊断邮件。",
	})
	if err != nil {
		fatalf("DeliveryEmail: %v", err)
	}

	fmt.Printf("投递成功: %s -> %s\n", *from, *to)
}

// splitList 按英文逗号切分并去掉空白项，空串返回 nil。
func splitList(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

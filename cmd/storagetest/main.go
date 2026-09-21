// 诊断程序：走真实七牛 Kodo 验证两种上传方式，并回读校验内容。
//
//	-mode=direct 前端直传：本地签名后以 multipart POST 到上传地址
//	-mode=server 服务端上传：走 provider.Upload 后回读 AccessURL
//
// 需要真实凭证且会上传文件到真实 bucket，故不放测试目录，由人工手动执行，非正式产物。
//
//	QINIU_ACCESS_KEY=xx QINIU_SECRET_KEY=xx \
//	  go run ./cmd/storagetest -mode=server -bucket=xxx -cdn-domain=xxx
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/ve-weiyi/vkit/adapter/storagex"
)

func main() {
	mode := flag.String("mode", "server", "上传方式：direct（前端直传）| server（服务端上传）")
	endpoint := flag.String("endpoint", "https://up-z2.qiniup.com", "上传 Endpoint")
	bucket := flag.String("bucket", "", "Bucket 名（必填）")
	region := flag.String("region", "huanan", "区域")
	cdnDomain := flag.String("cdn-domain", "", "CDN 域名")
	fileKey := flag.String("key", "vkit-storagetest.txt", "目标 fileKey")
	content := flag.String("content", "vkit storagetest content", "写入内容")
	isPrivate := flag.Bool("private", false, "是否私有空间")
	flag.Parse()

	if *bucket == "" {
		fatalf("必须指定 -bucket")
	}
	accessKey := os.Getenv("QINIU_ACCESS_KEY")
	secretKey := os.Getenv("QINIU_SECRET_KEY")
	if accessKey == "" || secretKey == "" {
		fatalf("请通过环境变量 QINIU_ACCESS_KEY / QINIU_SECRET_KEY 提供凭证")
	}

	provider := storagex.NewQiniuKodoProvider(&storagex.KodoConfig{
		Endpoint:  *endpoint,
		Bucket:    *bucket,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Region:    *region,
		CDNDomain: *cdnDomain,
		IsPrivate: *isPrivate,
	})

	var err error
	switch *mode {
	case "direct":
		err = directUpload(provider, *fileKey, []byte(*content))
	case "server":
		err = serverUpload(provider, *fileKey, []byte(*content))
	default:
		fatalf("未知的 -mode: %s（可选 direct | server）", *mode)
	}
	if err != nil {
		fatalf("%s 上传失败: %v", *mode, err)
	}

	fmt.Printf("上传成功: %s\n", provider.AccessURL(*fileKey))
}

// directUpload 模拟前端直传：本地签名后把 token 与文件一起 POST 到上传地址。
func directUpload(p *storagex.QiniuKodoProvider, fileKey string, content []byte) error {
	token, err := p.UploadToken(context.Background(), fileKey, 3600*time.Second)
	if err != nil {
		return fmt.Errorf("UploadToken: %w", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("token", token.Token); err != nil {
		return err
	}
	if err := writer.WriteField("key", token.FileKey); err != nil {
		return err
	}
	part, err := writer.CreateFormFile("file", fileKey)
	if err != nil {
		return err
	}
	if _, err := part.Write(content); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, token.UploadURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("状态 %d: %s", resp.StatusCode, respBody)
	}
	return nil
}

// serverUpload 走服务端上传，随后回读 AccessURL 校验内容一致。
func serverUpload(p *storagex.QiniuKodoProvider, fileKey string, content []byte) error {
	result, err := p.Upload(context.Background(), bytes.NewReader(content), fileKey)
	if err != nil {
		return fmt.Errorf("Upload: %w", err)
	}

	resp, err := http.Get(result.AccessURL)
	if err != nil {
		return fmt.Errorf("回读 %s: %w", result.AccessURL, err)
	}
	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, content) {
		return fmt.Errorf("内容不一致: 期望 %q，实际 %q", content, got)
	}
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

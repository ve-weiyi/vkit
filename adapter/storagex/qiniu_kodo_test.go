package storagex

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

var testQiniuCfg = &KodoConfig{
	Endpoint:  "https://up-z2.qiniup.com",
	Bucket:    "test-bucket",
	AccessKey: "test-access-key",
	SecretKey: "test-secret-key",
	Region:    "huanan",
	CDNDomain: "cdn.example.com",
}

// qiniuConfigFromEnv 读取真实上传测试所需配置；缺凭证时跳过，避免误用占位值访问真实 bucket。
func qiniuConfigFromEnv(t *testing.T) *KodoConfig {
	t.Helper()

	accessKey := os.Getenv("QINIU_ACCESS_KEY")
	secretKey := os.Getenv("QINIU_SECRET_KEY")
	bucket := os.Getenv("QINIU_BUCKET")
	if accessKey == "" || secretKey == "" || bucket == "" {
		t.Skip("跳过：未设置 QINIU_ACCESS_KEY / QINIU_SECRET_KEY / QINIU_BUCKET")
	}

	cfg := &KodoConfig{
		Endpoint:  "https://up-z2.qiniup.com",
		Bucket:    bucket,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Region:    "huanan",
		CDNDomain: os.Getenv("QINIU_CDN_DOMAIN"),
	}
	if v := os.Getenv("QINIU_ENDPOINT"); v != "" {
		cfg.Endpoint = v
	}
	if v := os.Getenv("QINIU_REGION"); v != "" {
		cfg.Region = v
	}

	return cfg
}

// newQiniuProvider 构造七牛 provider（构造失败即结束用例）。
func newQiniuProvider(tb testing.TB, cfg *KodoConfig) *QiniuKodoProvider {
	tb.Helper()
	p, err := NewQiniuKodoProvider(cfg)
	if err != nil {
		tb.Fatalf("NewQiniuKodoProvider: %v", err)
	}
	return p
}

func TestQiniuKodoProvider_UploadToken(t *testing.T) {
	provider := newQiniuProvider(t, testQiniuCfg)
	token, err := provider.UploadToken(context.Background(), "test-image.jpg", 3600*time.Second)
	if err != nil {
		t.Fatalf("UploadToken failed: %v", err)
	}

	if token.UploadURL == "" {
		t.Error("UploadURL should not be empty")
	}
	if token.Token == "" {
		t.Error("Token should not be empty")
	}
	if token.FileKey == "" {
		t.Error("FileKey should not be empty")
	}
	if token.AccessURL == "" {
		t.Error("AccessURL should not be empty")
	}
	if !strings.Contains(token.AccessURL, testQiniuCfg.CDNDomain) {
		t.Errorf("AccessURL should contain CDN domain, got: %s", token.AccessURL)
	}

	expectedExpire := time.Now().Add(3600 * time.Second)
	if token.ExpireAt.Before(expectedExpire.Add(-10*time.Second)) || token.ExpireAt.After(expectedExpire.Add(10*time.Second)) {
		t.Errorf("ExpireAt out of expected range, got: %v", token.ExpireAt)
	}
}

func TestQiniuKodoProvider_Region(t *testing.T) {
	regions := []struct {
		region     string
		expectHost string
	}{
		{"huadong", "up.qiniup.com"},
		{"huabei", "up-z1.qiniup.com"},
		{"huanan", "up-z2.qiniup.com"},
		{"beimei", "up-na0.qiniup.com"},
		{"xinjiapo", "up-as0.qiniup.com"},
	}

	for _, tc := range regions {
		t.Run(tc.region, func(t *testing.T) {
			provider := newQiniuProvider(t, &KodoConfig{
				Bucket: "test-bucket", AccessKey: "ak", SecretKey: "sk", Region: tc.region,
			})
			zone := provider.getRegion()
			if len(zone.SrcUpHosts) == 0 {
				t.Errorf("SrcUpHosts should not be empty for region: %s", tc.region)
			}
		})
	}
}

func TestQiniuKodoProvider_TokenFormat(t *testing.T) {
	provider := newQiniuProvider(t, testQiniuCfg)
	token, err := provider.UploadToken(context.Background(), "test.jpg", 3600*time.Second)
	if err != nil {
		t.Fatalf("UploadToken failed: %v", err)
	}

	parts := strings.Split(token.Token, ":")
	if len(parts) != 3 {
		t.Fatalf("Token should have 3 parts separated by ':', got %d", len(parts))
	}
	if parts[0] != testQiniuCfg.AccessKey {
		t.Errorf("AccessKey mismatch: expected %s, got %s", testQiniuCfg.AccessKey, parts[0])
	}
}

func TestQiniuKodoProvider_FileKeyGeneration(t *testing.T) {
	datePath := time.Now().Format("20060102")

	cases := []struct{ filename, ext string }{
		{"test.jpg", ".jpg"},
		{"image.png", ".png"},
		{"noext", ""},
	}

	for _, tc := range cases {
		t.Run(tc.filename, func(t *testing.T) {
			key := GenerateFileKey(tc.filename)
			if !strings.HasPrefix(key, datePath) {
				t.Errorf("FileKey should start with BasePath, got: %s", key)
			}
			if !strings.Contains(key, datePath) {
				t.Errorf("FileKey should contain date path %s, got: %s", datePath, key)
			}
			if tc.ext != "" && !strings.HasSuffix(key, tc.ext) {
				t.Errorf("FileKey should end with %s, got: %s", tc.ext, key)
			}
		})
	}
}

func TestQiniuKodoProvider_BuildAccessURL(t *testing.T) {
	cases := []struct {
		name      string
		cdnDomain string
		fileKey   string
		expected  string
	}{
		{"with CDN", "static.veweiyi.cn", "blog/20260202/test.jpg", "https://static.veweiyi.cn/blog/20260202/test.jpg"},
		{"without CDN", "", "test.jpg", "https://veweiyi.qiniucdn.com/test.jpg"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := newQiniuProvider(t, &KodoConfig{Bucket: "veweiyi", CDNDomain: tc.cdnDomain})
			if got := provider.AccessURL(tc.fileKey); got != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, got)
			}
		})
	}
}

// TestQiniuKodoProvider_RealUpload 测试前端直传，使用 -short 或未提供凭证时跳过
func TestQiniuKodoProvider_RealUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real upload test in short mode")
	}

	provider := newQiniuProvider(t, qiniuConfigFromEnv(t))
	token, err := provider.UploadToken(context.Background(), "test-upload.txt", 3600*time.Second)
	if err != nil {
		t.Fatalf("UploadToken failed: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("token", token.Token)
	_ = writer.WriteField("key", token.FileKey)
	part, err := writer.CreateFormFile("file", "test-upload.txt")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	_, _ = part.Write([]byte("test content"))
	_ = writer.Close()

	req, _ := http.NewRequest(http.MethodPost, token.UploadURL, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("upload failed: status=%d body=%s", resp.StatusCode, respBody)
	}
}

// TestQiniuKodoProvider_ServerSideUpload 测试服务端上传，使用 -short 或未提供凭证时跳过
func TestQiniuKodoProvider_ServerSideUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping server-side upload test in short mode")
	}

	provider := newQiniuProvider(t, qiniuConfigFromEnv(t))
	fileContent := []byte("server-side upload test content")

	result, err := provider.Upload(context.Background(), bytes.NewReader(fileContent), "server-test.txt")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	resp, err := http.Get(result.AccessURL)
	if err != nil {
		t.Logf("warning: failed to access uploaded file: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		content, _ := io.ReadAll(resp.Body)
		if !bytes.Equal(content, fileContent) {
			t.Errorf("content mismatch: expected %q, got %q", fileContent, content)
		}
	}
}

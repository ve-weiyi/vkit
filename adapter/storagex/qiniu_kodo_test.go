package storagex

import (
	"context"
	"strings"
	"testing"
	"time"
)

var testQiniuCfg = &KodoConfig{
	Endpoint:  "https://up-z2.qiniup.com",
	Bucket:    "veweiyi",
	AccessKey: "<qiniu-access-key>",
	SecretKey: "<qiniu-secret-key>",
	Region:    "huanan",
	CDNDomain: "static.veweiyi.cn",
	IsPrivate: false,
}

func TestQiniuKodoProvider_UploadToken(t *testing.T) {
	provider := NewQiniuKodoProvider(testQiniuCfg)
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
			provider := NewQiniuKodoProvider(&KodoConfig{
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
	provider := NewQiniuKodoProvider(testQiniuCfg)
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
			provider := NewQiniuKodoProvider(&KodoConfig{Bucket: "veweiyi", CDNDomain: tc.cdnDomain})
			if got := provider.AccessURL(tc.fileKey); got != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, got)
			}
		})
	}
}

func BenchmarkUploadToken(b *testing.B) {
	provider := NewQiniuKodoProvider(testQiniuCfg)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := provider.UploadToken(ctx, "test.jpg", 3600*time.Second); err != nil {
			b.Fatalf("UploadToken failed: %v", err)
		}
	}
}

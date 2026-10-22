package specification

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// 测试规格页解析：品牌、机型名、机型类别。
// 夹具为合成页面，只保证解析逻辑本身正确；对端页面是否改版由 cmd/webfetch 人工核验。
func TestSpecificationParsing(t *testing.T) {
	parse := func(t *testing.T, raw string) *goquery.Document {
		t.Helper()
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
		if err != nil {
			t.Fatalf("NewDocumentFromReader: %v", err)
		}
		return doc
	}

	t.Run("手机", func(t *testing.T) {
		doc := parse(t, `<html>
			<head><title>Apple iPhone 16 Pro Max - Full phone specifications</title></head>
			<body>
				<div class="section-heading"><a href="apple-phones-48.php">Popular from Apple</a></div>
				<h1 class="specs-phone-name-title">Apple iPhone 16 Pro Max</h1>
			</body></html>`)

		if got := getBrand(doc); got != "Apple" {
			t.Errorf("getBrand = %q, want %q", got, "Apple")
		}
		if got := getDeviceName(doc.Selection); got != "Apple iPhone 16 Pro Max" {
			t.Errorf("getDeviceName = %q, want %q", got, "Apple iPhone 16 Pro Max")
		}
		if got := getDeviceType(doc); got != Phone {
			t.Errorf("getDeviceType = %q, want %q", got, Phone)
		}
	})

	t.Run("平板", func(t *testing.T) {
		doc := parse(t, `<html>
			<head><title>Apple iPad Pro 11 - Full tablet specifications</title></head>
			<body></body></html>`)

		if got := getDeviceType(doc); got != Tablet {
			t.Errorf("getDeviceType = %q, want %q", got, Tablet)
		}
	})
}

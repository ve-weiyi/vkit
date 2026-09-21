package device

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// 测试机型条目解析：id / 名称 / slug / 图片 / 描述。
// 夹具为合成页面，只保证解析逻辑本身正确；对端页面是否改版由 cmd/webfetch 人工核验。
func TestNewDevice(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`
		<div class="makers"><ul><li>
			<a href="apple_iphone_16_pro_max-13123.php">
				<img src="https://cdn.example.com/iphone.jpg" title="Apple iPhone 16 Pro Max"/>
				<strong><span>Apple iPhone 16 Pro Max</span></strong>
			</a>
		</li></ul></div>`))
	if err != nil {
		t.Fatalf("NewDocumentFromReader: %v", err)
	}

	got, err := newDevice(doc.Find("a").First())
	if err != nil {
		t.Fatalf("newDevice: %v", err)
	}

	if got.ID != 13123 {
		t.Errorf("ID = %d, want %d", got.ID, 13123)
	}
	if got.Name != "Apple iPhone 16 Pro Max" {
		t.Errorf("Name = %q, want %q", got.Name, "Apple iPhone 16 Pro Max")
	}
	if got.Slug != "apple_iphone_16_pro_max-13123" {
		t.Errorf("Slug = %q, want %q", got.Slug, "apple_iphone_16_pro_max-13123")
	}
	if got.Image != "https://cdn.example.com/iphone.jpg" {
		t.Errorf("Image = %q", got.Image)
	}
	if got.Description != "Apple iPhone 16 Pro Max" {
		t.Errorf("Description = %q", got.Description)
	}

	t.Run("缺少 img 应报错", func(t *testing.T) {
		broken, err := goquery.NewDocumentFromReader(strings.NewReader(`
			<a href="apple_iphone_16_pro_max-13123.php">
				<strong><span>Apple iPhone 16 Pro Max</span></strong>
			</a>`))
		if err != nil {
			t.Fatalf("NewDocumentFromReader: %v", err)
		}
		if _, err := newDevice(broken.Find("a").First()); err == nil {
			t.Error("缺少 img 时应返回错误，实际为 nil")
		}
	})
}

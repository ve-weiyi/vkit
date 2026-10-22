package brand

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// 测试品牌条目解析：slug / id / 名称 / 机型数。
// 夹具为合成页面，只保证解析逻辑本身正确；对端页面是否改版由 cmd/webfetch 人工核验。
func TestNewBrand(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`
		<div class="brandmenu-v2"><ul><li>
			<a href="acer-phones-59.php">Acer<br/><span>123 devices</span></a>
		</li></ul></div>`))
	if err != nil {
		t.Fatalf("NewDocumentFromReader: %v", err)
	}

	got, err := newBrand(doc.Find("a").First())
	if err != nil {
		t.Fatalf("newBrand: %v", err)
	}

	if got.Slug != "acer-phones-59" {
		t.Errorf("Slug = %q, want %q", got.Slug, "acer-phones-59")
	}
	if got.ID != 59 {
		t.Errorf("ID = %d, want %d", got.ID, 59)
	}
	if got.Name != "Acer" {
		t.Errorf("Name = %q, want %q", got.Name, "Acer")
	}
	if got.NumberOfDevices != 123 {
		t.Errorf("NumberOfDevices = %d, want %d", got.NumberOfDevices, 123)
	}

	t.Run("缺少 href 应报错", func(t *testing.T) {
		broken, err := goquery.NewDocumentFromReader(strings.NewReader(`<a>Acer</a>`))
		if err != nil {
			t.Fatalf("NewDocumentFromReader: %v", err)
		}
		if _, err := newBrand(broken.Find("a").First()); err == nil {
			t.Error("缺少 href 时应返回错误，实际为 nil")
		}
	})
}

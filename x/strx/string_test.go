package strx

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExtractLetters(t *testing.T) {
	cases := []struct{ in, want string }{
		{"user_1_name-中", "username"},
		{"ABC", "ABC"},
		{"", ""},
		{"123-_", ""},
		{"中文", ""},
		{"a1B2c", "aBc"},
	}
	for _, c := range cases {
		if got := ExtractLetters(c.in); got != c.want {
			t.Errorf("ExtractLetters(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTruncateUTF8(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		maxBytes int
		want     string
	}{
		{"不足上限原样返回", "abc", 10, "abc"},
		{"恰好等于上限", "abc", 3, "abc"},
		{"ASCII 截断", "abcdef", 3, "abc"},
		{"非正数返回空串", "abc", 0, ""},
		{"负数返回空串", "abc", -1, ""},
		{"落在字符中间不切断", "中文键", 4, "中"}, // 中=3字节，第4字节是「文」的首字节
		{"落在字符边界", "中文键", 6, "中文"},
		{"全部保留", "中文键", 9, "中文键"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := TruncateUTF8(c.in, c.maxBytes)
			if got != c.want {
				t.Errorf("TruncateUTF8(%q, %d) = %q, want %q", c.in, c.maxBytes, got, c.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("TruncateUTF8(%q, %d) = %q 不是合法 UTF-8", c.in, c.maxBytes, got)
			}
			if !strings.HasPrefix(c.in, got) {
				t.Errorf("TruncateUTF8(%q, %d) = %q 不是原串前缀", c.in, c.maxBytes, got)
			}
		})
	}

	// 穷举所有截断长度，任意长度都必须产出合法 UTF-8 前缀
	t.Run("穷举不切断", func(t *testing.T) {
		s := "a中b文c键d"
		for n := 0; n <= len(s)+2; n++ {
			got := TruncateUTF8(s, n)
			if !utf8.ValidString(got) || !strings.HasPrefix(s, got) {
				t.Fatalf("TruncateUTF8(%q, %d) = %q 非法", s, n, got)
			}
			if len(got) > n {
				t.Fatalf("TruncateUTF8(%q, %d) = %q 超出上限", s, n, got)
			}
		}
	})
}

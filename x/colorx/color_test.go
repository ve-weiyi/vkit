package colorx

import (
	"strings"
	"testing"
)

// 着色函数的契约：把原串包进 ANSI 转义序列，且不丢失原串
func TestColor(t *testing.T) {
	cases := []struct {
		name string
		fn   func(string) string
	}{
		{"Red", Red},
		{"Blue", Blue},
		{"Green", Green},
	}

	for _, c := range cases {
		got := c.fn("hello")
		if !strings.Contains(got, "hello") {
			t.Errorf("%s(\"hello\") = %q，未包含原串", c.name, got)
		}
		if !strings.HasPrefix(got, "\x1b[") {
			t.Errorf("%s(\"hello\") = %q，未以 ANSI 转义开头", c.name, got)
		}
		if got == "hello" {
			t.Errorf("%s(\"hello\") 未着色", c.name)
		}
	}
}

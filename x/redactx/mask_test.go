package redactx_test

import (
	"testing"

	"github.com/ve-weiyi/vkit/x/redactx"
)

func TestMaskPhone(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"13812345678", "138****5678"},
		{"我的手机号是13812345678", "我的手机号是138****5678"},
		{"no-phone-here", "no-phone-here"},
	}
	for _, c := range cases {
		if got := redactx.MaskPhone(c.in); got != c.want {
			t.Errorf("redactx.MaskPhone(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMaskEmail(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"username@qq.com", "ue******@qq.com"},
		{"我的邮箱号是username@qq.com", "我的邮箱号是ue******@qq.com"},
		// 用户名不足 3 位：原样返回，且不能丢掉域名
		{"ab@qq.com", "ab@qq.com"},
		{"no-email-here", "no-email-here"},
	}
	for _, c := range cases {
		if got := redactx.MaskEmail(c.in); got != c.want {
			t.Errorf("redactx.MaskEmail(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

package validatex_test

import (
	"testing"

	"github.com/ve-weiyi/vkit/x/validatex"
)

func TestIsEmail(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"user@example.com", true},
		{"user.name+tag@sub.example.co", true},
		{"user@example", false},
		{"@example.com", false},
		{"user name@example.com", false},
		{"", false},
	}
	for _, c := range cases {
		if got := validatex.IsEmail(c.in); got != c.want {
			t.Errorf("IsEmail(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIsMobile(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"13812345678", true},
		{"19900000000", true},
		{"12812345678", false},
		{"1381234567", false},
		{"138123456789", false},
		{"", false},
	}
	for _, c := range cases {
		if got := validatex.IsMobile(c.in); got != c.want {
			t.Errorf("IsMobile(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

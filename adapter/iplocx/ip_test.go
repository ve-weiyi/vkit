package iplocx

import "testing"

// 测试 IP 归一化的纯逻辑：去端口与本地地址判定
func TestStripPortAndIsLocalhost(t *testing.T) {
	t.Run("stripPort", func(t *testing.T) {
		cases := []struct {
			in   string
			want string
		}{
			{"1.2.3.4:80", "1.2.3.4"},
			{"1.2.3.4", "1.2.3.4"},
			{"[::1]:80", "::1"},
			{"", ""},
		}

		for _, c := range cases {
			if got := stripPort(c.in); got != c.want {
				t.Errorf("stripPort(%q) = %q, want %q", c.in, got, c.want)
			}
		}
	})

	t.Run("isLocalhost", func(t *testing.T) {
		cases := []struct {
			in   string
			want bool
		}{
			{"localhost", true},
			{"127.0.0.1", true},
			{"::1", true},
			{"8.8.8.8", false},
			{"not-an-ip", false},
			{"", false},
		}

		for _, c := range cases {
			if got := isLocalhost(c.in); got != c.want {
				t.Errorf("isLocalhost(%q) = %v, want %v", c.in, got, c.want)
			}
		}
	})
}

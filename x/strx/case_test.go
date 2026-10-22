package strx

import (
	"testing"
)

// TestToSnakeToCamel 定值表取自 jsonconv 旧键名实现（regex 改写序列化产物）的输出，
// 用于锁定改造后 ASCII 路径的行为完全不变。
func TestToSnakeToCamel(t *testing.T) {
	cases := []struct{ in, snake, camel string }{
		{"UserID", "user_id", "userID"},
		{"NickName", "nick_name", "nickName"},
		{"ID", "i_d", "iD"},
		{"userID", "user_id", "userID"},
		{"user_id", "user_id", "userId"},
		{"userID2Name", "user_id2_name", "userID2Name"},
		{"HTTPServer", "h_tt_pserver", "hTTPServer"},
		{"ABC", "a_bc", "aBC"},
		{"a", "a", "a"},
		{"A", "a", "a"},
		{"user_name", "user_name", "userName"},
		{"_lead", "_lead", "lead"},
		{"tail_", "tail_", "tail"},
		{"a__b", "a__b", "aB"},
		{"URLPath", "u_rl_path", "uRLPath"},
		{"IPAddr", "i_paddr", "iPAddr"},
		{"user1Name", "user1_name", "user1Name"},
		{"user_1id", "user_1id", "user1Id"},
		{"JSONData", "j_so_ndata", "jSONData"},
		{"XMLHTTPRequest", "x_ml_ht_tp_request", "xMLHTTPRequest"},
		{"already_snake_case", "already_snake_case", "alreadySnakeCase"},
		{"single", "single", "single"},
		{"SINGLE", "s_in_gl_e", "sINGLE"},
		{"U", "u", "u"},
		{"s", "s", "s"},
		{"aB", "a_b", "aB"},
		{"Ab", "ab", "ab"},
		{"A_B", "a__b", "aB"},
		{"userIDList", "user_id_list", "userIDList"},
		{"a1b2C3", "a1b2_c3", "a1b2C3"},
		{"", "", ""},
	}

	for _, c := range cases {
		if got := ToSnake(c.in); got != c.snake {
			t.Errorf("ToSnake(%q) = %q, want %q", c.in, got, c.snake)
		}
		if got := ToCamel(c.in); got != c.camel {
			t.Errorf("ToCamel(%q) = %q, want %q", c.in, got, c.camel)
		}
	}
}

// TestCaseConversionKeepsMultibyte 回归：旧实现按字节遍历，
// 会把多字节字符截成单字节（Std.ToCamel("中文_键") 只剩 "\xe4\xe6\xe9"）。
func TestCaseConversionKeepsMultibyte(t *testing.T) {
	cases := []struct{ in, snake, camel string }{
		{"中文键", "中文键", "中文键"},
		{"用户_名", "用户_名", "用户名"},
		{"café_name", "café_name", "caféName"},
		{"名前", "名前", "名前"},
	}
	for _, c := range cases {
		if got := ToSnake(c.in); got != c.snake {
			t.Errorf("ToSnake(%q) = %q, want %q", c.in, got, c.snake)
		}
		if got := ToCamel(c.in); got != c.camel {
			t.Errorf("ToCamel(%q) = %q, want %q", c.in, got, c.camel)
		}
	}
}

func TestFirstUpperLower(t *testing.T) {
	cases := []struct{ in, upper, lower string }{
		{"abc", "Abc", "abc"},
		{"ABC", "ABC", "aBC"},
		{"", "", ""},
		{"1a", "1a", "1a"},
		{"_a", "_a", "_a"},
		{"中文", "中文", "中文"}, // 多字节字符原样返回，不产生替换符
		{"éa", "éa", "éa"},
	}
	for _, c := range cases {
		if got := FirstUpper(c.in); got != c.upper {
			t.Errorf("FirstUpper(%q) = %q, want %q", c.in, got, c.upper)
		}
		if got := FirstLower(c.in); got != c.lower {
			t.Errorf("FirstLower(%q) = %q, want %q", c.in, got, c.lower)
		}
	}
}

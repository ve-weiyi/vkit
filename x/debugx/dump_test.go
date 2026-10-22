package debugx

import (
	"testing"
)

func TestDump(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{
			name: "结构体输出缩进 JSON",
			in:   struct{ UserID int }{UserID: 1},
			want: "{\n \"UserID\": 1\n}",
		},
		{
			name: "map 输出缩进 JSON",
			in:   map[string]any{"a": []int{1, 2}},
			want: "{\n \"a\": [\n  1,\n  2\n ]\n}",
		},
		{
			name: "nil 指针",
			in:   (*struct{})(nil),
			want: "null",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Dump(c.in); got != c.want {
				t.Errorf("Dump() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestDumpFallbacks 序列化结果为空对象时回落到 %+v，
// 以便调试时能看出字段名与真实 Go 类型。
func TestDumpFallbacks(t *testing.T) {
	t.Run("空 map 回落成 %+v", func(t *testing.T) {
		if got := Dump(map[string]any{}); got != "map[]" {
			t.Errorf("Dump(空 map) = %q, want %q", got, "map[]")
		}
	})

	t.Run("空结构体回落成 %+v", func(t *testing.T) {
		if got := Dump(struct{}{}); got != "{}" {
			t.Errorf("Dump(空结构体) = %q, want %q", got, "{}")
		}
	})
}

// TestDumpMarshalFailure 序列化失败时打印错误并返回空串 ——
// 这是沿用自 jsonconv.AnyToJsonIndent 的既有行为，调用方拿不到失败信号，
// 因此 Dump 只能用于调试，不能产出交给下游解析的数据。
func TestDumpMarshalFailure(t *testing.T) {
	got := Dump(func() {})
	if got != "" {
		t.Errorf("Dump(不可序列化值) = %q, want 空串", got)
	}
}

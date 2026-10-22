package dstx

import (
	"reflect"
	"testing"
)

// 测试字面量类型推断：整数优先于浮点，带引号才视作字符串
func TestInferType(t *testing.T) {
	cases := []struct {
		in   string
		want interface{}
	}{
		{"11", 11},
		{"-3", -3},
		{"11.0", 11.0},
		{"1e3", 1000.0},
		{`"22"`, "22"},
		{"abc", "abc"},
		{"", ""},
	}

	for _, c := range cases {
		got, err := inferType(c.in)
		if err != nil {
			t.Errorf("inferType(%q) 返回错误: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("inferType(%q) = %v (%T), want %v (%T)", c.in, got, got, c.want, c.want)
		}
	}
}

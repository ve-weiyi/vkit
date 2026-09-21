package jsonv

import (
	"io"
	"os"
	"strings"
	"testing"
)

type person struct {
	UserID   int               `json:"user_id"`
	NickName string            `json:"nickName"`
	Tags     []string          `json:"tags,omitempty"`
	Meta     map[string]string `json:"meta"`
}

func sample() person {
	return person{UserID: 1, NickName: "n", Tags: []string{"a"}, Meta: map[string]string{"k": "v"}}
}

// noError 把不支持序列化的值，用于覆盖失败分支。
func unsupported() any { return func() {} }

// captureStdout 捕获 fn 执行期间写入标准输出的内容。
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()
	w.Close()

	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(b)
}

func TestAnyToJson(t *testing.T) {
	t.Run("成功", func(t *testing.T) {
		got, err := AnyToJson(sample())
		if err != nil {
			t.Fatalf("AnyToJson() error = %v", err)
		}
		if want := `{"user_id":1,"nickName":"n","tags":["a"],"meta":{"k":"v"}}`; got != want {
			t.Errorf("AnyToJson() = %s, want %s", got, want)
		}
	})

	t.Run("失败返回错误", func(t *testing.T) {
		if _, err := AnyToJson(unsupported()); err == nil {
			t.Error("AnyToJson() 应当返回错误")
		}
	})
}

func TestAnyToJsonNE(t *testing.T) {
	t.Run("成功", func(t *testing.T) {
		if got, want := AnyToJsonNE(sample()), `{"user_id":1,"nickName":"n","tags":["a"],"meta":{"k":"v"}}`; got != want {
			t.Errorf("AnyToJsonNE() = %s, want %s", got, want)
		}
	})

	t.Run("失败返回空串并留下提示", func(t *testing.T) {
		var got string
		out := captureStdout(t, func() { got = AnyToJsonNE(unsupported()) })
		if got != "" {
			t.Errorf("AnyToJsonNE() = %q, want 空串", got)
		}
		if !strings.Contains(out, "AnyToJsonNE:json convert fail:") {
			t.Errorf("标准输出 = %q, want 含失败提示", out)
		}
	})
}

func TestAnyToAny(t *testing.T) {
	t.Run("结构体转 map", func(t *testing.T) {
		var got map[string]any
		if err := AnyToAny(sample(), &got); err != nil {
			t.Fatalf("AnyToAny() error = %v", err)
		}
		if got["user_id"] != float64(1) || got["nickName"] != "n" {
			t.Errorf("AnyToAny() = %+v", got)
		}
	})

	t.Run("map 转结构体", func(t *testing.T) {
		var got person
		if err := AnyToAny(map[string]any{"user_id": 2, "nickName": "z"}, &got); err != nil {
			t.Fatalf("AnyToAny() error = %v", err)
		}
		if got.UserID != 2 || got.NickName != "z" {
			t.Errorf("AnyToAny() = %+v", got)
		}
	})

	t.Run("失败返回错误", func(t *testing.T) {
		var got map[string]any
		if err := AnyToAny(unsupported(), &got); err == nil {
			t.Error("AnyToAny() 应当返回错误")
		}
	})
}

func TestAnyToAnyNE(t *testing.T) {
	t.Run("成功返回指针", func(t *testing.T) {
		got := AnyToAnyNE[person](sample())
		if got == nil {
			t.Fatal("AnyToAnyNE() = nil")
		}
		if got.UserID != 1 || got.NickName != "n" {
			t.Errorf("AnyToAnyNE() = %+v", *got)
		}
	})

	t.Run("失败返回 nil 并留下提示", func(t *testing.T) {
		var got *person
		out := captureStdout(t, func() { got = AnyToAnyNE[person](unsupported()) })
		if got != nil {
			t.Errorf("AnyToAnyNE() = %+v, want nil", got)
		}
		if !strings.Contains(out, "AnyToAnyNE:json convert fail:") {
			t.Errorf("标准输出 = %q, want 含失败提示", out)
		}
	})
}

func TestJsonToAny(t *testing.T) {
	t.Run("成功", func(t *testing.T) {
		var got person
		if err := JsonToAny(`{"user_id":3,"nickName":"x"}`, &got); err != nil {
			t.Fatalf("JsonToAny() error = %v", err)
		}
		if got.UserID != 3 || got.NickName != "x" {
			t.Errorf("JsonToAny() = %+v", got)
		}
	})

	t.Run("空串视为无内容，不改动目标", func(t *testing.T) {
		got := person{UserID: 9, NickName: "keep"}
		if err := JsonToAny("", &got); err != nil {
			t.Fatalf("JsonToAny() error = %v", err)
		}
		if got.UserID != 9 || got.NickName != "keep" {
			t.Errorf("JsonToAny(\"\") 改动了目标: %+v", got)
		}
	})

	t.Run("非法 JSON 返回错误", func(t *testing.T) {
		var got person
		if err := JsonToAny(`{bad`, &got); err == nil {
			t.Error("JsonToAny() 应当返回错误")
		}
	})
}

func TestJsonToAnyNE(t *testing.T) {
	t.Run("成功", func(t *testing.T) {
		if got := JsonToAnyNE[person](`{"user_id":4,"nickName":"y"}`); got.UserID != 4 || got.NickName != "y" {
			t.Errorf("JsonToAnyNE() = %+v", got)
		}
	})

	t.Run("空串返回零值", func(t *testing.T) {
		if got := JsonToAnyNE[person](""); got.UserID != 0 || got.NickName != "" {
			t.Errorf("JsonToAnyNE(\"\") = %+v, want 零值", got)
		}
	})

	t.Run("非法 JSON 返回零值并留下提示", func(t *testing.T) {
		var got person
		out := captureStdout(t, func() { got = JsonToAnyNE[person](`{bad`) })
		if got.UserID != 0 || got.NickName != "" {
			t.Errorf("JsonToAnyNE(bad) = %+v, want 零值", got)
		}
		if !strings.Contains(out, "JsonToAnyNE:json convert fail:") {
			t.Errorf("标准输出 = %q, want 含失败提示", out)
		}
	})
}

func TestMapConversions(t *testing.T) {
	t.Run("AnyToMap", func(t *testing.T) {
		got, err := AnyToMap(sample())
		if err != nil {
			t.Fatalf("AnyToMap() error = %v", err)
		}
		if got["user_id"] != float64(1) {
			t.Errorf("AnyToMap() = %+v", got)
		}
	})

	t.Run("AnyToMap 失败返回错误", func(t *testing.T) {
		if _, err := AnyToMap(unsupported()); err == nil {
			t.Error("AnyToMap() 应当返回错误")
		}
	})

	t.Run("AnyToMapNE 失败返回 nil", func(t *testing.T) {
		var got map[string]any
		out := captureStdout(t, func() { got = AnyToMapNE(unsupported()) })
		if got != nil {
			t.Errorf("AnyToMapNE() = %+v, want nil", got)
		}
		if !strings.Contains(out, "AnyToMapNE:json convert fail:") {
			t.Errorf("标准输出 = %q, want 含失败提示", out)
		}
	})

	t.Run("JsonToMap", func(t *testing.T) {
		got, err := JsonToMap([]byte(`{"a":1}`))
		if err != nil {
			t.Fatalf("JsonToMap() error = %v", err)
		}
		if got["a"] != float64(1) {
			t.Errorf("JsonToMap() = %+v", got)
		}
	})

	t.Run("JsonToMap 非法 JSON 返回错误", func(t *testing.T) {
		if _, err := JsonToMap([]byte(`{bad`)); err == nil {
			t.Error("JsonToMap() 应当返回错误")
		}
	})

	t.Run("JsonToMapNE 非法 JSON 返回 nil", func(t *testing.T) {
		var got map[string]any
		out := captureStdout(t, func() { got = JsonToMapNE([]byte(`{bad`)) })
		if got != nil {
			t.Errorf("JsonToMapNE() = %+v, want nil", got)
		}
		if !strings.Contains(out, "JsonToMapNE:json convert fail:") {
			t.Errorf("标准输出 = %q, want 含失败提示", out)
		}
	})
}

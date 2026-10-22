package jsonv

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJsonSnakeCaseMarshal(t *testing.T) {
	type order struct {
		UserID   int
		NickName string
	}
	got, err := json.Marshal(JsonSnakeCase{Value: order{UserID: 1, NickName: "n"}})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if want := `{"nick_name":"n","user_id":1}`; string(got) != want {
		t.Errorf("json.Marshal(JsonSnakeCase) = %s, want %s", got, want)
	}
}

func TestJsonCamelCaseMarshal(t *testing.T) {
	type order struct {
		UserID   int
		NickName string
	}
	got, err := json.Marshal(JsonCamelCase{Value: order{UserID: 1, NickName: "n"}})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if want := `{"nickName":"n","userID":1}`; string(got) != want {
		t.Errorf("json.Marshal(JsonCamelCase) = %s, want %s", got, want)
	}
}

func TestAnyToJsonSnakeCamel(t *testing.T) {
	cases := []struct {
		name  string
		in    any
		snake string
		camel string
	}{
		{
			name:  "平铺",
			in:    map[string]any{"userID": 1, "nickName": "n"},
			snake: `{"nick_name":"n","user_id":1}`,
			camel: `{"nickName":"n","userID":1}`,
		},
		{
			name:  "嵌套对象",
			in:    map[string]any{"outerKey": map[string]any{"innerKey": 1}},
			snake: `{"outer_key":{"inner_key":1}}`,
			camel: `{"outerKey":{"innerKey":1}}`,
		},
		{
			name:  "数组内的对象",
			in:    map[string]any{"itemList": []any{map[string]any{"subKey": 1}}},
			snake: `{"item_list":[{"sub_key":1}]}`,
			camel: `{"itemList":[{"subKey":1}]}`,
		},
		{
			name:  "标量数组原样",
			in:    map[string]any{"tagList": []any{1, "a", true, nil}},
			snake: `{"tag_list":[1,"a",true,null]}`,
			camel: `{"tagList":[1,"a",true,null]}`,
		},
		{
			name:  "中文键不参与转换也不被破坏",
			in:    map[string]any{"中文键": 1},
			snake: `{"中文键":1}`,
			camel: `{"中文键":1}`,
		},
		{
			name:  "id 键无需特例",
			in:    map[string]any{"id": 1},
			snake: `{"id":1}`,
			camel: `{"id":1}`,
		},
		{
			name:  "大整数不丢精度",
			in:    map[string]any{"bigID": int64(9007199254740993)},
			snake: `{"big_id":9007199254740993}`,
			camel: `{"bigID":9007199254740993}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AnyToJsonSnake(c.in); got != c.snake {
				t.Errorf("AnyToJsonSnake() = %s, want %s", got, c.snake)
			}
			if got := AnyToJsonCamel(c.in); got != c.camel {
				t.Errorf("AnyToJsonCamel() = %s, want %s", got, c.camel)
			}
		})
	}
}

// TestKeyCaseSortsKeys 键顺序由 json.Marshal 决定，为字典序。
// JSON 对象的键顺序无语义，此处只是把行为固定下来。
func TestKeyCaseSortsKeys(t *testing.T) {
	got := AnyToJsonSnake(map[string]any{"bKey": 2, "aKey": 1})
	if want := `{"a_key":1,"b_key":2}`; got != want {
		t.Errorf("AnyToJsonSnake() = %s, want %s", got, want)
	}
}

func TestKeyCaseError(t *testing.T) {
	for _, fn := range []struct {
		name string
		call func() string
	}{
		{"AnyToJsonSnake", func() string { return AnyToJsonSnake(unsupported()) }},
		{"AnyToJsonCamel", func() string { return AnyToJsonCamel(unsupported()) }},
	} {
		t.Run(fn.name, func(t *testing.T) {
			var got string
			out := captureStdout(t, func() { got = fn.call() })
			if got != "" {
				t.Errorf("%s() = %q, want 空串", fn.name, got)
			}
			if !strings.Contains(out, fn.name+":json convert fail:") {
				t.Errorf("标准输出 = %q, want 含失败提示", out)
			}
		})
	}
}

// TestSnakeCaseOutputIsValidJSON 输出必须能被标准库解析回等价结构。
func TestSnakeCaseOutputIsValidJSON(t *testing.T) {
	in := map[string]any{"outerKey": map[string]any{"innerKey": []any{1, 2}}}
	out := AnyToJsonSnake(in)

	var back map[string]any
	if err := json.Unmarshal([]byte(out), &back); err != nil {
		t.Fatalf("输出不是合法 JSON: %v (%s)", err, out)
	}
	outer, ok := back["outer_key"].(map[string]any)
	if !ok {
		t.Fatalf("缺少 outer_key: %s", out)
	}
	if _, ok := outer["inner_key"]; !ok {
		t.Errorf("缺少 inner_key: %s", out)
	}
}

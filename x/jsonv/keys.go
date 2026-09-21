package jsonv

import (
	"bytes"
	"encoding/json"

	"github.com/ve-weiyi/vkit/x/strx"
)

// 序列化时改写对象键的包装类型。
//
// 实现方式：先按标准库序列化，再把结果解析成通用结构、逐层改写其中的对象键，
// 最后重新序列化。
// 三点需要留意：
//   - 对象的键顺序变为字典序（JSON 对象的键顺序无语义）；
//   - 数字按 json.Number 原样保留，避免大整数经 float64 往返丢精度；
//   - 两个不同的键归一化后同名时（如同时存在 userID 与 user_id）只会留下一个。
//     这种输入本身就是歧义的，调用方应避免。

// JsonSnakeCase 序列化时把对象的键（含嵌套）转成下划线风格。
//
//	json.Marshal(jsonconv.JsonSnakeCase{Value: v})  // {"user_id":1}
type JsonSnakeCase struct {
	Value any
}

// MarshalJSON 实现 json.Marshaler。
func (c JsonSnakeCase) MarshalJSON() ([]byte, error) {
	return marshalWithKeyCase(c.Value, strx.ToSnake)
}

// JsonCamelCase 序列化时把对象的键（含嵌套）转成小驼峰风格。
//
//	json.Marshal(jsonconv.JsonCamelCase{Value: v})  // {"userId":1}
type JsonCamelCase struct {
	Value any
}

// MarshalJSON 实现 json.Marshaler。
func (c JsonCamelCase) MarshalJSON() ([]byte, error) {
	return marshalWithKeyCase(c.Value, strx.ToCamel)
}

// AnyToJsonSnake 序列化为 JSON 字符串，键转为下划线风格，失败时返回空串。
func AnyToJsonSnake(data any) string {
	jb, err := json.Marshal(JsonSnakeCase{Value: data})
	if err != nil {
		reportNE("AnyToJsonSnake", err)
		return ""
	}
	return string(jb)
}

// AnyToJsonCamel 序列化为 JSON 字符串，键转为小驼峰风格，失败时返回空串。
func AnyToJsonCamel(data any) string {
	jb, err := json.Marshal(JsonCamelCase{Value: data})
	if err != nil {
		reportNE("AnyToJsonCamel", err)
		return ""
	}
	return string(jb)
}

// marshalWithKeyCase 序列化 v，并把所有对象（含嵌套）的键交给 convert 改写。
func marshalWithKeyCase(v any, convert func(string) string) ([]byte, error) {
	jb, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(jb))
	dec.UseNumber()

	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}

	return json.Marshal(rewriteKeys(tree, convert))
}

// rewriteKeys 深度改写对象键；数组递归处理，标量原样返回。
func rewriteKeys(v any, convert func(string) string) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[convert(k)] = rewriteKeys(val, convert)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = rewriteKeys(val, convert)
		}
		return out
	default:
		return v
	}
}

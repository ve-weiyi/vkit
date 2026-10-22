package jsonv

// 任意类型之间的 JSON 转换。
//
// *NE 系列（No Error）按既有语义忽略错误并返回零值，供不关心失败的分支使用；
// 新代码请优先使用返回 error 的版本。

// AnyToJson 序列化为 JSON 字符串。
func AnyToJson(data any) (string, error) {
	return encodeToString(data)
}

// AnyToJsonNE 同 AnyToJson，失败时返回空串。
//
// Deprecated: 失败与「结果本身是空串」无法区分，调用方无从感知。请改用 AnyToJson。
func AnyToJsonNE(data any) string {
	s, err := encodeToString(data)
	if err != nil {
		reportNE("AnyToJsonNE", err)
		return ""
	}
	return s
}

// AnyToAny 把 data 序列化后反序列化到 obj，obj 必须是指针。
func AnyToAny(data any, obj any) error {
	return encodeInto(data, obj)
}

// AnyToAnyNE 同 AnyToAny，但反序列化结果由返回值给出，失败时返回 nil。
//
// Deprecated: 失败与「转换结果为空」无法区分，调用方无从感知。请改用 AnyToAny。
func AnyToAnyNE[T any](data any) (out *T) {
	out = new(T)
	if err := encodeInto(data, out); err != nil {
		reportNE("AnyToAnyNE", err)
		return nil
	}
	return out
}

// JsonToAny 把 JSON 字符串反序列化到 obj，obj 必须是指针。空串视为无内容。
func JsonToAny(js string, obj any) error {
	return decodeString(js, obj)
}

// JsonToAnyNE 同 JsonToAny，结果由返回值给出，失败时返回零值。
//
// Deprecated: 失败与「转换结果为零值」无法区分，调用方无从感知。请改用 JsonToAny。
func JsonToAnyNE[T any](js string) (out T) {
	if err := decodeString(js, &out); err != nil {
		reportNE("JsonToAnyNE", err)
		return out
	}
	return out
}

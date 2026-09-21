package jsonv

// 与 map[string]any 之间的转换。

// AnyToMap 把 obj 序列化后反序列化为 map。
func AnyToMap(obj any) (map[string]any, error) {
	var result map[string]any
	if err := encodeInto(obj, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// AnyToMapNE 同 AnyToMap，失败时返回 nil。
//
// Deprecated: 失败与「转换结果为空 map」无法区分，调用方无从感知。请改用 AnyToMap。
func AnyToMapNE(obj any) map[string]any {
	var result map[string]any
	if err := encodeInto(obj, &result); err != nil {
		reportNE("AnyToMapNE", err)
		return nil
	}
	return result
}

// JsonToMap 把 JSON 字节切片反序列化为 map。
func JsonToMap(data []byte) (map[string]any, error) {
	var result map[string]any
	if err := decodeBytes(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// JsonToMapNE 同 JsonToMap，失败时返回 nil。
//
// Deprecated: 失败与「转换结果为空 map」无法区分，调用方无从感知。请改用 JsonToMap。
func JsonToMapNE(data []byte) map[string]any {
	var result map[string]any
	if err := decodeBytes(data, &result); err != nil {
		reportNE("JsonToMapNE", err)
		return nil
	}
	return result
}

package jsonv

import (
	"encoding/json"
	"fmt"
)

// 本包所有导出函数共用的编解码出口，负责统一错误包装，
// 以及 *NE 系列的错误提示格式。

// encodeToString 序列化为 JSON 字符串。
func encodeToString(v any) (string, error) {
	jb, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(jb), nil
}

// encodeInto 序列化 v 后反序列化到 obj，obj 必须是指针。
func encodeInto(v any, obj any) error {
	jb, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(jb, obj)
}

// decodeString 反序列化 JSON 字符串。空串视为无内容，直接成功返回。
func decodeString(data string, obj any) error {
	if data == "" {
		return nil
	}
	return decodeBytes([]byte(data), obj)
}

// decodeBytes 反序列化 JSON 字节切片。
func decodeBytes(data []byte, obj any) error {
	return json.Unmarshal(data, obj)
}

// reportNE 报告一次被忽略的转换错误。*NE 系列不返回错误，
// 按既有语义只在标准输出留下痕迹；需要感知失败请改用返回 error 的版本。
func reportNE(name string, err error) {
	fmt.Println(name+":json convert fail:", err)
}

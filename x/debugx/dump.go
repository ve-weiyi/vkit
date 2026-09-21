// Package debugx 提供调试打印辅助。
package debugx

import (
	"encoding/json"
	"fmt"
)

// Dump 把 v 转成便于人工阅读的字符串：优先输出缩进 JSON；
// 序列化结果为空对象时回落到 %+v，以便看出字段名与真实 Go 类型。
//
// 仅供调试，不要用它的返回值产出需要下游解析的数据：
// 序列化失败时它会打印错误并返回空串，调用方拿不到失败信号。
func Dump(v any) string {
	jb, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		fmt.Println("debugx.Dump: json convert fail:", err)
		return ""
	}
	if string(jb) == "{}" {
		return fmt.Sprintf("%+v", v)
	}
	return string(jb)
}

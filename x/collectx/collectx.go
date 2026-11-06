// Package collectx 提供列表字段提取的泛型辅助。
package collectx

// Distinct 按 key 提取值，去重并过滤空串，保持首次出现顺序。
func Distinct[T any](list []T, key func(T) string) []string {
	seen := make(map[string]struct{}, len(list))
	result := make([]string, 0, len(list))

	for _, item := range list {
		v := key(item)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
}

// DistinctMany 对每个元素提取多个值，去重并过滤空串，保持首次出现顺序。
func DistinctMany[T any](list []T, key func(T) []string) []string {
	seen := make(map[string]struct{}, len(list))
	result := make([]string, 0, len(list))

	for _, item := range list {
		for _, v := range key(item) {
			if v == "" {
				continue
			}
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

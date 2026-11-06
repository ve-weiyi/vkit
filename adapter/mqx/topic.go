package mqx

import "strings"

// MatchTopic 判断 routing key 是否匹配绑定模式。
//
// 语义与 AMQP topic exchange 一致：* 匹配一段，# 匹配零到多段。
// 支持服务端绑定模式的后端由 broker 过滤；其余后端用它做客户端过滤。
func MatchTopic(pattern, key string) bool {
	return matchParts(strings.Split(pattern, "."), strings.Split(key, "."))
}

func matchParts(pattern, key []string) bool {
	if len(pattern) == 0 {
		return len(key) == 0
	}
	if pattern[0] == "#" {
		if len(pattern) == 1 {
			return true
		}
		for i := 0; i <= len(key); i++ {
			if matchParts(pattern[1:], key[i:]) {
				return true
			}
		}
		return false
	}
	if len(key) == 0 {
		return false
	}
	if pattern[0] == "*" || pattern[0] == key[0] {
		return matchParts(pattern[1:], key[1:])
	}
	return false
}

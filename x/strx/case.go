// Package strx 提供字符串处理工具。
//
// 所有函数按 rune 处理，不会把多字节字符截成单字节。
// 命名转换只识别 ASCII 字母 —— Go 标识符命名约定本身就是 ASCII 的，
// 非 ASCII 字符原样保留，既不参与转换也不被破坏。
package strx

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ToSnake 驼峰转下划线，按「词边界」规则插入下划线，匹配非重叠、从左到右：
//
//	UserID   -> user_id
//	NickName -> nick_name
//	ID       -> i_d
//	user_id  -> user_id
//
// 与 jsonconv 的键名转换行为一致：只在 ASCII 字母数字与紧跟其后的 ASCII 大写字母之间断词。
func ToSnake(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)

	for i := 0; i < len(s); {
		c := s[i]
		if c >= utf8.RuneSelf {
			_, size := utf8.DecodeRuneInString(s[i:])
			b.WriteString(s[i : i+size])
			i += size
			continue
		}

		b.WriteByte(lowerASCII(c))
		// 当前字符与其后的大写字母构成词边界：两个字符一并消费，
		// 因此 ID -> i_d 而不是 i__d（非重叠匹配）。
		if isWordASCII(c) && i+1 < len(s) && isUpperASCII(s[i+1]) {
			b.WriteByte('_')
			b.WriteByte(lowerASCII(s[i+1]))
			i += 2
			continue
		}
		i++
	}

	return b.String()
}

// ToCamel 下划线转驼峰（小驼峰），以非字母字符为分隔：
//
//	user_id   -> userId
//	nick_name -> nickName
//	userName  -> userName
//
// 分隔后遇到的第一个字母转大写，其余字符保持原样；数字不改变分隔状态
// （与既有行为一致：user_1id -> user1Id）。
func ToCamel(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	afterSeparator := true
	for _, r := range s {
		if r >= utf8.RuneSelf {
			if unicode.IsLetter(r) {
				afterSeparator = false
				b.WriteRune(r)
			} else {
				afterSeparator = true
			}
			continue
		}

		switch {
		case r >= '0' && r <= '9':
			b.WriteByte(byte(r))
		case r >= 'a' && r <= 'z':
			if afterSeparator {
				b.WriteByte(byte(r) - ('a' - 'A'))
			} else {
				b.WriteByte(byte(r))
			}
			afterSeparator = false
		case r >= 'A' && r <= 'Z':
			b.WriteByte(byte(r))
			afterSeparator = false
		default:
			afterSeparator = true
		}
	}

	return FirstLower(b.String())
}

// FirstUpper 首字母转大写，其余字符原样保留。
func FirstUpper(s string) string {
	if s == "" {
		return ""
	}
	if c := s[0]; c >= 'a' && c <= 'z' {
		return string(c-('a'-'A')) + s[1:]
	}
	return s
}

// FirstLower 首字母转小写，其余字符原样保留。
func FirstLower(s string) string {
	if s == "" {
		return ""
	}
	if c := s[0]; c >= 'A' && c <= 'Z' {
		return string(c+('a'-'A')) + s[1:]
	}
	return s
}

func isWordASCII(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}

func isUpperASCII(c byte) bool {
	return c >= 'A' && c <= 'Z'
}

func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

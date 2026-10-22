package strx

import (
	"strings"
	"unicode/utf8"
)

// ExtractLetters 取出字符串中的 ASCII 字母，其余字符丢弃。
func ExtractLetters(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); {
		c := s[i]
		if c >= utf8.RuneSelf {
			_, size := utf8.DecodeRuneInString(s[i:])
			i += size
			continue
		}
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			b.WriteByte(c)
		}
		i++
	}

	return b.String()
}

// TruncateUTF8 把字符串按字节截断到最多 maxBytes 字节，且不切断多字节字符。
// maxBytes 小于等于 0 时返回空串。
func TruncateUTF8(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}

	// 回退到最后一个完整的 rune 边界
	for maxBytes > 0 && !utf8.RuneStart(s[maxBytes]) {
		maxBytes--
	}

	return s[:maxBytes]
}

package maskx

import (
	"regexp"
)

// 正则提到包级：写在函数体内会每次调用重新编译
var (
	phonePattern = regexp.MustCompile(`(\d{3})\d{4}(\d{4})`)
	emailPattern = regexp.MustCompile(`([\w._%+-]+)(@[\w.-]+\.[A-Za-z]{2,})`)
)

// MaskPhone 手机号脱敏 13812345678 --> 138****5678
func MaskPhone(phone string) string {
	return phonePattern.ReplaceAllString(phone, "$1****$2")
}

// MaskEmail 邮箱脱敏 username@qq.com --> ue******@qq.com
func MaskEmail(email string) string {
	return emailPattern.ReplaceAllStringFunc(email, func(s string) string {
		matches := emailPattern.FindStringSubmatch(s)
		if len(matches) != 3 {
			return s
		}
		username := matches[1]
		domain := matches[2]

		// 用户名不足 3 位时没有可遮的位置，原样返回（旧实现这里只返回用户名，把域名丢了）
		runes := []rune(username)
		length := len(runes)
		if length <= 2 {
			return s
		}

		masked := make([]rune, length)
		masked[0] = runes[0]
		masked[1] = runes[2]
		for i := 2; i < length; i++ {
			masked[i] = '*'
		}
		return string(masked) + domain
	})
}

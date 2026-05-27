package jsonconv

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const (
	Camel = 0 //驼峰
	Case  = 1 //下划线
)

// commonInitialisms from https://github.com/golang/lint/blob/master/lint.go#L770
var commonInitialisms = []string{
	"API", "ASCII", "CPU", "CSS", "DNS", "EOF", "GUID", "HTML", "HTTP",
	"HTTPS", "ID", "IP", "JSON", "LHS", "QPS", "RAM", "RHS", "RPC",
	"SLA", "SMTP", "SSH", "TLS", "TTL", "UID", "UI", "UUID", "URI",
	"URL", "UTF8", "VM", "XML", "XSRF", "XSS",

	// 项目自定义
	"AI", "DTO", "PID",
}

// sortedInitialisms sorted by length descending so longer initialisms match first
var sortedInitialisms []string

func init() {
	sortedInitialisms = make([]string, len(commonInitialisms))
	copy(sortedInitialisms, commonInitialisms)
	sort.Slice(sortedInitialisms, func(i, j int) bool {
		return len(sortedInitialisms[i]) > len(sortedInitialisms[j])
	})
}

/**
 * 驼峰式写法转为下划线写法
 * @description XxYx->xx_yy	XxYY->xx_yy	EncodeURL->url  TagDTOList->tag_dto_list
 **/
func Case2Snake(XxYY string) string {
	xx_y_y := make([]byte, 0)
	i := 0

	for i < len(XxYY) {
		flag := false
		// 非首个字符
		if len(xx_y_y) != 0 {
			// 方案二:自动匹配。如果前一个字符是小写 || 后一个字符是小写
			//if (0 < i-1 && unicode.IsLower(rune(XxYY[i-1]))) || (i+1 < len(XxYY) && unicode.IsLower(rune(XxYY[i+1]))) {
			//	xx_y_y = append(xx_y_y, '_')
			//}
			flag = true
		}

		// 未找到 replaceKey ，进行正常转换
		w := rune(XxYY[i])
		i++
		// 遇到数字
		if unicode.IsDigit(w) {
			xx_y_y = append(xx_y_y, byte(w))
			continue
		}
		// 遇到非字母
		if !unicode.IsLetter(w) {
			xx_y_y = append(xx_y_y, byte('_'))
			continue
		}
		// 如果是大写
		if unicode.IsUpper(w) {
			if flag {
				xx_y_y = append(xx_y_y, '_')
			}
			xx_y_y = append(xx_y_y, byte(unicode.ToLower(w)))
		} else {
			xx_y_y = append(xx_y_y, byte(w))
		}
	}

	result := string(xx_y_y)
	// Fix broken initialisms: "user_i_d" → "user_id" (replicate gorm toDBName behavior)
	for _, init := range sortedInitialisms {
		brokenForm := strings.ToLower(strings.Join(strings.Split(init, ""), "_"))
		re := regexp.MustCompile(`(^|_)` + regexp.QuoteMeta(brokenForm) + `(_|$)`)
		result = re.ReplaceAllString(result, `${1}`+strings.ToLower(init)+`${2}`)
	}
	return result
}

/**
 * 下划线转驼峰
 * @description xx_yy to XxYx  xx_y_y to XxYY  XxYY to XxYY
 * @date 2023/2/15
 * @param xx_y_y
 * @return XxYY
 **/
func Case2Camel(xx_y_y string) string {
	//id类型转换大写
	XxYY := make([]byte, 0, len(xx_y_y))
	//是否遇到下划线,初始化值为true则转换第一个字母
	line := true
	i := 0
	for i < len(xx_y_y) {
		// 未找到 replaceKey ，进行正常转换
		w := rune(xx_y_y[i])
		i++
		//遇到数字
		if unicode.IsDigit(w) {
			XxYY = append(XxYY, byte(w))
			continue
		}

		//遇到 _
		if !unicode.IsLetter(w) {
			line = true
			continue
		}

		//遇到小写
		if w >= 'a' && w <= 'z' {
			if line {
				w = w - 32
			}
		}
		//遇到大写，跳过
		if w >= 'A' && w <= 'Z' {

		}
		//只对 _ 后一个字母生效
		if line {
			line = false
		}
		XxYY = append(XxYY, byte(w))
	}

	result := string(XxYY[:])
	// Fix Go initialisms: "UserId" → "UserID", "IpAddress" → "IPAddress" (replicate gorm toSchemaName)
	for _, initialism := range sortedInitialisms {
		titleInit := strings.ToUpper(initialism[:1]) + strings.ToLower(initialism[1:])
		result = regexp.MustCompile(titleInit+`([A-Z]|$|_)`).ReplaceAllString(result, initialism+"$1")
	}
	return result
}

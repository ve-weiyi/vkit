package httpx

import (
	"io"
	"sort"
	"strings"
)

// CURL 生成等价的 curl 命令，用于调试。
func (r *Request) CURL() string {
	var parts []string
	parts = append(parts, "curl", "-X", r.Method)

	keys := make([]string, 0, len(r.Header))
	for k := range r.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		for _, v := range r.Header[k] {
			parts = append(parts, "-H", shellQuote(k+": "+v))
		}
	}

	// 包含请求体
	if r.GetBody != nil {
		bodyReader, err := r.GetBody()
		if err != nil {
			parts = append(parts, "# [failed to replay body: "+err.Error()+"]")
		} else {
			bodyBytes, readErr := io.ReadAll(bodyReader)
			bodyReader.Close()
			if readErr == nil && len(bodyBytes) > 0 {
				parts = append(parts, "--data-raw", shellQuote(string(bodyBytes)))
			}
		}
	}

	parts = append(parts, shellQuote(r.URL.String()))

	return strings.Join(parts, " ")
}

// shellQuote 用单引号包裹参数，并转义内部单引号，保证输出可在 shell 中安全执行。
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

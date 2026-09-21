package randomx

import (
	"crypto/rand"
	"fmt"
	"math/big"
	mathrand "math/rand/v2"
	"strings"
	"time"

	"github.com/google/uuid"
)

// cryptoInt 取 [0, n) 的密码学安全随机数。
// 安全场景（验证码、单号）必须走这里：math/rand 的输出可由少量样本推算出来。
func cryptoInt(n int64) (int, error) {
	v, err := rand.Int(rand.Reader, big.NewInt(n))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

// 生成随机数字账号，不以 0 开头。
// 非安全场景，用 math/rand/v2：它是并发安全的，且无需处理错误。
func GenerateQQNumber() string {
	length := mathrand.IntN(2) + 7 // 7~8 位
	digits := make([]byte, length)

	// 首位不能为 0
	digits[0] = byte(mathrand.IntN(9)+1) + '0'

	for i := 1; i < length; i++ {
		digits[i] = byte(mathrand.IntN(10)) + '0'
	}
	return string(digits)
}

func GenerateRandomUUID() string {
	return strings.Replace(uuid.New().String(), "-", "", -1)
}

// 生成指定长度的纯数字随机字符串。
// 用于验证码等安全场景，随机数取自 crypto/rand，不可预测。
func GenerateCode(length int) (string, error) {
	const charset = "0123456789"

	// 预分配字节切片，避免扩容
	result := make([]byte, length)
	for i := range result {
		n, err := cryptoInt(int64(len(charset)))
		if err != nil {
			return "", fmt.Errorf("randomx: 生成验证码失败: %w", err)
		}
		result[i] = charset[n]
	}
	return string(result), nil
}

// GenerateOrderNo 生成唯一订单号
// 格式: PAY + 时间戳(14位) + 随机数(6位)
// 示例: PAY20260204153045123456
func GenerateOrderNo() (string, error) {
	return merchantNo("PAY")
}

// GenerateTransactionNo 生成唯一流水号
// 格式: TXN + 时间戳(14位) + 随机数(6位)
// 示例: TXN20260204153045123456
func GenerateTransactionNo() (string, error) {
	return merchantNo("TXN")
}

// merchantNo 拼出「前缀 + 时间戳 + 6 位随机数」的单号。
// 随机数取自 crypto/rand：时间戳是公开可推的，单号的唯一性全靠这一段。
func merchantNo(prefix string) (string, error) {
	timestamp := time.Now().Format("20060102150405")

	n, err := cryptoInt(1000000)
	if err != nil {
		return "", fmt.Errorf("randomx: 生成单号失败: %w", err)
	}

	return fmt.Sprintf("%s%s%06d", prefix, timestamp, n), nil
}

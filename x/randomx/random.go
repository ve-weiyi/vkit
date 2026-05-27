package randomx

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
)

// 初始化随机种子
var r = rand.New(rand.NewSource(time.Now().UnixNano()))

// 生成随机数字账号，不以 0 开头
func GenerateQQNumber() string {
	length := r.Intn(2) + 7 // 7~8 位
	digits := make([]byte, length)

	// 首位不能为 0
	digits[0] = byte(rand.Intn(9)+1) + '0'

	for i := 1; i < length; i++ {
		digits[i] = byte(rand.Intn(10)) + '0'
	}
	return string(digits)
}

func GenerateRandomUUID() string {
	return strings.Replace(uuid.New().String(), "-", "", -1)
}

// 生成指定长度的纯数字随机字符串
func GenerateCode(length int) string {
	const charset = "0123456789"

	// 预分配字节切片，避免扩容
	result := make([]byte, length)
	for i := range result {
		// 直接从字符集中随机选取，逻辑更直观
		result[i] = charset[r.Intn(len(charset))]
	}
	return string(result)
}

// GenerateOrderNo 生成唯一订单号
// 格式: PAY + 时间戳(14位) + 随机数(6位)
// 示例: PAY20260204153045123456
func GenerateOrderNo() string {
	// 时间戳部分: YYYYMMDDHHmmss
	timestamp := time.Now().Format("20060102150405")

	// 随机数部分: 6位随机数
	random := rand.Intn(1000000)

	return fmt.Sprintf("PAY%s%06d", timestamp, random)
}

// GenerateTransactionNo 生成唯一流水号
// 格式: TXN + 时间戳(14位) + 随机数(6位)
// 示例: TXN20260204153045123456
func GenerateTransactionNo() string {
	// 时间戳部分: YYYYMMDDHHmmss
	timestamp := time.Now().Format("20060102150405")

	// 随机数部分: 6位随机数
	random := rand.Intn(1000000)

	return fmt.Sprintf("TXN%s%06d", timestamp, random)
}

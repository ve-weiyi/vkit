package moneyx

import (
	"fmt"
	"math"
)

// Money 金额类型，单位：分
type Money int64

// FromYuan 从元转换为分
func FromYuan(yuan float64) Money {
	return Money(math.Round(yuan * 100))
}

// ToYuan 从分转换为元
func (m Money) ToYuan() float64 {
	return float64(m) / 100.0
}

// Add 加法
func (m Money) Add(other Money) Money {
	return m + other
}

// Sub 减法
func (m Money) Sub(other Money) Money {
	return m - other
}

// IsPositive 是否为正数
func (m Money) IsPositive() bool {
	return m > 0
}

// IsNegative 是否为负数
func (m Money) IsNegative() bool {
	return m < 0
}

// String 字符串表示（元）
func (m Money) String() string {
	return fmt.Sprintf("%.2f", m.ToYuan())
}

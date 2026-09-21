package mqx

import (
	"fmt"

	"github.com/zeromicro/go-zero/core/logx"
)

// LogxLogger 将 go-zero 的 logx.Logger 适配为 mqx.Logger，
// 使 rabbitmqx / redisstreamx / kafkax 内部的运行错误（如重连失败、拓扑声明失败）
// 能通过 logx 输出，避免因 Logger 为 nil 而静默吞掉错误。
type LogxLogger struct {
	Logger logx.Logger
}

// Info 实现 mqx.Logger，将 key-value 对转为 logx 结构化字段。
func (l *LogxLogger) Info(msg string, keysAndValues ...interface{}) {
	if l.Logger == nil {
		return
	}
	l.Logger.Infow(msg, toLogFields(keysAndValues...)...)
}

// Error 实现 mqx.Logger。
func (l *LogxLogger) Error(msg string, keysAndValues ...interface{}) {
	if l.Logger == nil {
		return
	}
	l.Logger.Errorw(msg, toLogFields(keysAndValues...)...)
}

// toLogFields 将 [k1, v1, k2, v2, ...] 转为 []logx.LogField；奇数尾项忽略。
func toLogFields(keysAndValues ...interface{}) []logx.LogField {
	fields := make([]logx.LogField, 0, len(keysAndValues)/2)
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		fields = append(fields, logx.Field(fmt.Sprint(keysAndValues[i]), keysAndValues[i+1]))
	}
	return fields
}

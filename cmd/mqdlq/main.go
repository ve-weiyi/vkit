// mqdlq 是死信队列的人工检查与重放工具。
//
// 默认只列出消息；确认后用 -replay -yes 重放。缺凭证时显式失败，不回落默认值。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

const maxReplayCount = 3

func main() {
	url := flag.String("url", os.Getenv("MQX_RABBIT_URL"), "AMQP URL（默认读取 MQX_RABBIT_URL）")
	queue := flag.String("queue", "", "死信队列名")
	limit := flag.Int("limit", 100, "最多检查多少条消息")
	eventID := flag.String("event-id", "", "只重放指定 event_id")
	all := flag.Bool("all", false, "重放检查到的全部消息")
	replay := flag.Bool("replay", false, "真正重放；缺省只列出")
	yes := flag.Bool("yes", false, "确认重放")
	force := flag.Bool("force", false, "重放次数已达上限时仍强制重放")
	destination := flag.String("destination", "", "原目的地头缺失时的兜底 exchange")
	routingKey := flag.String("routing-key", "", "原 routing key 头缺失时的兜底值")
	flag.Parse()

	if *url == "" {
		fail("-url 或 MQX_RABBIT_URL 必填")
	}
	if *queue == "" {
		fail("-queue 必填")
	}
	if *replay && !*yes {
		fail("-replay 需要同时指定 -yes")
	}
	if *replay && !*all && *eventID == "" {
		fail("-replay 需要 -all 或 -event-id")
	}

	conn, err := amqp.Dial(*url)
	if err != nil {
		fail("连接 RabbitMQ 失败: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		fail("创建 channel 失败: %v", err)
	}
	defer func() { _ = ch.Close() }()

	if *limit <= 0 {
		fail("-limit 必须为正数")
	}
	if *limit > 1000 {
		fmt.Printf("limit capped from %d to 1000\n", *limit)
		*limit = 1000
	}

	var returns chan amqp.Return
	if *replay {
		if err := ch.Confirm(false); err != nil {
			fail("开启 publisher confirm 失败: %v", err)
		}
		returns = ch.NotifyReturn(make(chan amqp.Return, 4))
	}

	// 暂不 requeue，避免单条消息在 Get 循环里被反复取到；结束时统一放回。
	pending := make([]amqp.Delivery, 0, *limit)
	inspected := 0
	replayed := 0
	for inspected < *limit {
		delivery, ok, err := ch.Get(*queue, false)
		if err != nil {
			fail("读取死信失败: %v", err)
		}
		if !ok {
			break
		}
		inspected++
		printDelivery(inspected, delivery)

		if !*replay || !matches(delivery, *all, *eventID) {
			pending = append(pending, delivery)
			continue
		}

		if err := replayOne(ch, returns, delivery, *destination, *routingKey, *force); err != nil {
			_ = delivery.Nack(false, true)
			fail("重放失败: %v", err)
		}
		if err := delivery.Ack(false); err != nil {
			fail("确认原死信失败: %v", err)
		}
		replayed++
		fmt.Println("  replayed")
	}

	for _, delivery := range pending {
		if err := delivery.Nack(false, true); err != nil {
			fail("放回未重放消息失败: %v", err)
		}
	}
	fmt.Printf("inspected %d messages, replayed %d\n", inspected, replayed)
}

func matches(delivery amqp.Delivery, all bool, eventID string) bool {
	if all {
		return true
	}
	return headerString(delivery.Headers, mqx.HeaderEventID) == eventID
}

func replayOne(ch *amqp.Channel, returns <-chan amqp.Return, delivery amqp.Delivery, fallbackDestination, fallbackRoutingKey string, force bool) error {
	destination := headerString(delivery.Headers, mqx.HeaderOriginalDestination)
	if destination == "" {
		destination = fallbackDestination
	}
	routingKey := headerString(delivery.Headers, mqx.HeaderOriginalRoutingKey)
	if routingKey == "" {
		routingKey = fallbackRoutingKey
	}
	if destination == "" {
		return fmt.Errorf("missing original destination; pass -destination")
	}

	replays := parseReplayCount(delivery.Headers)
	if replays >= maxReplayCount && !force {
		return fmt.Errorf("replay_count=%d reached limit; pass -force to override", replays)
	}

	headers := make(amqp.Table, len(delivery.Headers)+2)
	for key, value := range delivery.Headers {
		headers[key] = value
	}
	headers["mqx.replayed_at"] = time.Now().Format(time.RFC3339Nano)
	headers["mqx.replay_count"] = replays + 1

	messageID := delivery.MessageId
	if messageID == "" {
		messageID = headerString(delivery.Headers, mqx.HeaderEventID)
	}

	drainReturns(returns)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	confirm, err := ch.PublishWithDeferredConfirmWithContext(ctx, destination, routingKey, true, false, amqp.Publishing{
		MessageId:   messageID,
		ContentType: delivery.ContentType,
		Timestamp:   time.Now(),
		Headers:     headers,
		Body:        delivery.Body,
	})
	if err != nil {
		return err
	}
	if confirm == nil {
		return fmt.Errorf("publisher confirm unavailable")
	}
	acked, err := confirm.WaitContext(ctx)
	if err != nil {
		return err
	}

	// mandatory=true 时，原队列不存在会触发 basic.return，但 confirm 仍可能 ack。
	if returns != nil {
		select {
		case returned, ok := <-returns:
			if ok {
				return fmt.Errorf("unroutable: exchange=%s routing_key=%s: %s", returned.Exchange, returned.RoutingKey, returned.ReplyText)
			}
		default:
		}
	}
	if !acked {
		return fmt.Errorf("broker rejected replay")
	}
	return nil
}

func drainReturns(returns <-chan amqp.Return) {
	if returns == nil {
		return
	}
	for {
		select {
		case _, ok := <-returns:
			if !ok {
				return
			}
		default:
			return
		}
	}
}

func printDelivery(index int, delivery amqp.Delivery) {
	eventID := headerString(delivery.Headers, mqx.HeaderEventID)
	routingKey := headerString(delivery.Headers, mqx.HeaderOriginalRoutingKey)
	if routingKey == "" {
		routingKey = delivery.RoutingKey
	}
	attempts := headerString(delivery.Headers, mqx.HeaderAttempts)
	failedAt := headerString(delivery.Headers, mqx.HeaderFailedAt)
	errText := headerString(delivery.Headers, mqx.HeaderError)
	if len(errText) > 120 {
		errText = errText[:120] + "..."
	}

	fmt.Printf("[%d] event_id=%s routing_key=%s attempts=%s failed_at=%s body=%dB\n",
		index, eventID, routingKey, attempts, failedAt, len(delivery.Body))
	if errText != "" {
		fmt.Printf("    error=%s\n", errText)
	}
}

func headerString(headers amqp.Table, key string) string {
	value, ok := headers[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func parseReplayCount(headers amqp.Table) int {
	value := headerString(headers, "mqx.replay_count")
	if value == "" {
		return 0
	}
	count, err := strconv.Atoi(value)
	if err != nil || count < 0 {
		return 0
	}
	return count
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

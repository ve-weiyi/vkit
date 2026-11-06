//go:build integration

package mqtest

import (
	"os"
	"strings"
	"testing"

	"github.com/ve-weiyi/vkit/adapter/mqx"
	"github.com/ve-weiyi/vkit/adapter/mqx/impl/kafkax"
)

// TestKafkaConformance 需要真实 Kafka：
//
//	MQX_KAFKA_SEEDS=localhost:9092 go test -tags integration ./adapter/mqx/mqtest/
func TestKafkaConformance(t *testing.T) {
	seeds := os.Getenv("MQX_KAFKA_SEEDS")
	if seeds == "" {
		t.Skip("MQX_KAFKA_SEEDS not set")
	}
	runConformance(t, func(t *testing.T) mqx.Broker {
		broker, err := kafkax.NewBroker(&kafkax.Config{
			Seeds: strings.Split(seeds, ","),
		})
		if err != nil {
			t.Fatalf("NewBroker: %v", err)
		}
		return broker
	})
}

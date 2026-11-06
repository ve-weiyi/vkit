//go:build integration

package mqtest

import (
	"os"
	"testing"
	"time"

	"github.com/ve-weiyi/vkit/adapter/mqx"
	"github.com/ve-weiyi/vkit/adapter/mqx/impl/redisstreamx"
)

// TestRedisConformance 需要真实 Redis：
//
//	MQX_REDIS_ADDR=localhost:6379 go test -tags integration ./adapter/mqx/mqtest/
func TestRedisConformance(t *testing.T) {
	addr := os.Getenv("MQX_REDIS_ADDR")
	if addr == "" {
		t.Skip("MQX_REDIS_ADDR not set")
	}
	runConformance(t, func(t *testing.T) mqx.Broker {
		broker, err := redisstreamx.NewBroker(&redisstreamx.Config{
			Addrs:     []string{addr},
			ClaimIdle: time.Second,
		})
		if err != nil {
			t.Fatalf("NewBroker: %v", err)
		}
		return broker
	})
}

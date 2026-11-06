package mqtest

import (
	"testing"

	"github.com/ve-weiyi/vkit/adapter/mqx"
	"github.com/ve-weiyi/vkit/adapter/mqx/impl/memmqx"
)

func TestMemConformance(t *testing.T) {
	runConformance(t, func(t *testing.T) mqx.Broker {
		return memmqx.NewBroker(memmqx.Config{})
	})
}

package mqtest

import (
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/twmb/franz-go/pkg/kerr"

	"github.com/ve-weiyi/vkit/adapter/mqx"
	"github.com/ve-weiyi/vkit/adapter/mqx/impl/kafkax"
	"github.com/ve-weiyi/vkit/adapter/mqx/impl/rabbitmqx"
	"github.com/ve-weiyi/vkit/adapter/mqx/impl/redisstreamx"
)

type publishErrorCase struct {
	name      string
	err       error
	permanent bool
}

func TestPublishErrorClassification(t *testing.T) {
	t.Run("rabbit", func(t *testing.T) {
		runPublishErrorCases(t, rabbitmqx.ClassifyPublishError, []publishErrorCase{
			{"access refused", &amqp.Error{Code: 403, Reason: "ACCESS_REFUSED"}, true},
			{"not found", &amqp.Error{Code: 404, Reason: "NOT_FOUND"}, true},
			{"resource locked", &amqp.Error{Code: 405, Reason: "RESOURCE_LOCKED"}, true},
			{"precondition failed", &amqp.Error{Code: 406, Reason: "PRECONDITION_FAILED"}, true},
			{"internal error", &amqp.Error{Code: 500, Reason: "INTERNAL_ERROR"}, false},
			{"network", errors.New("connection reset"), false},
		})
	})

	t.Run("kafka", func(t *testing.T) {
		runPublishErrorCases(t, kafkax.ClassifyPublishError, []publishErrorCase{
			{"topic authorization", kerr.TopicAuthorizationFailed, true},
			{"message too large", kerr.MessageTooLarge, true},
			{"unknown topic", kerr.UnknownTopicOrPartition, false},
			{"unknown server", kerr.UnknownServerError, false},
			{"network", errors.New("connection reset"), false},
		})
	})

	t.Run("redis", func(t *testing.T) {
		runPublishErrorCases(t, redisstreamx.ClassifyPublishError, []publishErrorCase{
			{"no auth", testRedisError("NOAUTH Authentication required"), true},
			{"no perm", testRedisError("NOPERM this user has no permissions"), true},
			{"wrong type", testRedisError("WRONGTYPE Operation against a key holding the wrong kind of value"), true},
			{"invalid command", testRedisError("ERR wrong number of arguments"), true},
			{"network", errors.New("connection refused"), false},
		})
	})
}

func runPublishErrorCases(t *testing.T, classify func(error) error, cases []publishErrorCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			classified := classify(tc.err)
			if classified == nil {
				t.Fatalf("classify(%v) = nil", tc.err)
			}
			if got := mqx.IsPermanent(classified); got != tc.permanent {
				t.Fatalf("IsPermanent = %v, want %v (err = %v)", got, tc.permanent, classified)
			}
		})
	}
}

type testRedisError string

func (e testRedisError) Error() string { return string(e) }
func (testRedisError) RedisError()     {}

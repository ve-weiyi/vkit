package event

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/ve-weiyi/vkit/adapter/mqx"
)

// tracingConfig 是发布/消费两侧共用的调用链配置。
type tracingConfig struct {
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
	system     string
}

func defaultTracingConfig() tracingConfig {
	return tracingConfig{
		tracer: otel.Tracer("mqx"),
		propagator: propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, propagation.Baggage{},
		),
		system: "mqx",
	}
}

func (c tracingConfig) attributes(destination, operation, id string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("messaging.system", c.system),
		attribute.String("messaging.destination.name", destination),
		attribute.String("messaging.operation.name", operation),
		attribute.String("messaging.message.id", id),
	}
}

type tracedPublisher struct {
	next mqx.Publisher
	cfg  tracingConfig
}

var _ mqx.BatchPublisher = (*tracedPublisher)(nil)

func wrapPublisher(next mqx.Publisher, cfg tracingConfig) mqx.Publisher {
	if next == nil {
		return nil
	}
	return &tracedPublisher{next: next, cfg: cfg}
}

func (p *tracedPublisher) Publish(ctx context.Context, msg *mqx.Message) (*mqx.PublishResult, error) {
	if msg == nil {
		return p.next.Publish(ctx, msg)
	}

	ctx, span := p.cfg.tracer.Start(ctx, "messaging.publish",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(p.cfg.attributes(msg.Destination, "publish", msg.ID)...),
	)
	defer span.End()

	headers := make(map[string]string, len(msg.Headers)+1)
	for k, v := range msg.Headers {
		headers[k] = v
	}
	p.cfg.propagator.Inject(ctx, propagation.MapCarrier(headers))

	cloned := *msg
	cloned.Headers = headers

	result, err := p.next.Publish(ctx, &cloned)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if result != nil && result.ID != "" {
		span.SetAttributes(attribute.String("messaging.message.id", result.ID))
	}
	return result, nil
}

func (p *tracedPublisher) PublishBatch(ctx context.Context, msgs []*mqx.Message) ([]mqx.PublishResult, error) {
	batch, ok := p.next.(mqx.BatchPublisher)
	if !ok {
		return nil, mqx.ErrNotSupported
	}

	cloned := make([]*mqx.Message, len(msgs))
	spans := make([]trace.Span, len(msgs))
	for i, msg := range msgs {
		if msg == nil {
			continue
		}
		spanCtx, span := p.cfg.tracer.Start(ctx, "messaging.publish",
			trace.WithSpanKind(trace.SpanKindProducer),
			trace.WithAttributes(p.cfg.attributes(msg.Destination, "publish", msg.ID)...),
		)
		headers := make(map[string]string, len(msg.Headers)+1)
		for k, v := range msg.Headers {
			headers[k] = v
		}
		p.cfg.propagator.Inject(spanCtx, propagation.MapCarrier(headers))
		copied := *msg
		copied.Headers = headers
		cloned[i] = &copied
		spans[i] = span
	}
	defer func() {
		for _, span := range spans {
			if span != nil {
				span.End()
			}
		}
	}()

	results, err := batch.PublishBatch(ctx, cloned)
	if err != nil {
		for _, span := range spans {
			if span != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
		}
	}
	return results, err
}

func wrapHandler(next mqx.Handler, cfg tracingConfig) mqx.Handler {
	return func(ctx context.Context, d *mqx.Delivery) error {
		ctx = cfg.propagator.Extract(ctx, propagation.MapCarrier(d.Headers))
		ctx, span := cfg.tracer.Start(ctx, "messaging.process",
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(append(
				cfg.attributes(d.Destination, "process", d.ID),
				attribute.Int("messaging.message.delivery_attempt", d.Attempt),
			)...),
		)
		defer span.End()

		err := next(ctx, d)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return err
	}
}

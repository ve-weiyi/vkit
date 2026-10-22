package httpx

import (
	"context"
	"testing"
)

func TestChain(t *testing.T) {
	t.Run("empty chain calls final handler", func(t *testing.T) {
		called := false
		final := func(ctx context.Context, req *Request) (*Response, error) {
			called = true
			return &Response{}, nil
		}

		if _, err := Chain()(final)(context.Background(), &Request{}); err != nil {
			t.Fatalf("Chain() error: %v", err)
		}
		if !called {
			t.Error("final handler 未被调用")
		}
	})

	t.Run("executes in onion order", func(t *testing.T) {
		var order []string

		middleware := func(name string) Middleware {
			return func(next Handler) Handler {
				return func(ctx context.Context, req *Request) (*Response, error) {
					order = append(order, name+"-before")
					resp, err := next(ctx, req)
					order = append(order, name+"-after")
					return resp, err
				}
			}
		}

		final := func(ctx context.Context, req *Request) (*Response, error) {
			order = append(order, "final")
			return &Response{}, nil
		}

		composed := Chain(middleware("m1"), middleware("m2"))(final)
		if _, err := composed(context.Background(), &Request{}); err != nil {
			t.Fatalf("composed error: %v", err)
		}

		want := []string{"m1-before", "m2-before", "final", "m2-after", "m1-after"}
		if len(order) != len(want) {
			t.Fatalf("events = %v, want %v", order, want)
		}
		for i := range want {
			if order[i] != want[i] {
				t.Errorf("position %d = %q, want %q", i, order[i], want[i])
			}
		}
	})

	t.Run("propagates context", func(t *testing.T) {
		type ctxKey struct{}

		m := func(next Handler) Handler {
			return func(ctx context.Context, req *Request) (*Response, error) {
				return next(context.WithValue(ctx, ctxKey{}, "from-middleware"), req)
			}
		}
		final := func(ctx context.Context, req *Request) (*Response, error) {
			if got := ctx.Value(ctxKey{}); got != "from-middleware" {
				t.Errorf("context value = %v, want from-middleware", got)
			}
			return &Response{}, nil
		}

		if _, err := Chain(m)(final)(context.Background(), &Request{}); err != nil {
			t.Fatalf("Chain error: %v", err)
		}
	})
}

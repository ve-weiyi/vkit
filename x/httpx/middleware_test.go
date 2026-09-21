package httpx

import (
	"context"
	"testing"
)

func TestChain(t *testing.T) {
	var order []string

	m1 := func(next Handler) Handler {
		return func(ctx context.Context, req *Request) (*Response, error) {
			order = append(order, "m1-before")
			resp, err := next(ctx, req)
			order = append(order, "m1-after")
			return resp, err
		}
	}

	m2 := func(next Handler) Handler {
		return func(ctx context.Context, req *Request) (*Response, error) {
			order = append(order, "m2-before")
			resp, err := next(ctx, req)
			order = append(order, "m2-after")
			return resp, err
		}
	}

	final := func(ctx context.Context, req *Request) (*Response, error) {
		order = append(order, "final")
		return &Response{}, nil
	}

	composed := Chain(m1, m2)(final)
	_, err := composed(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"m1-before", "m2-before", "final", "m2-after", "m1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d events, got %d: %v", len(expected), len(order), order)
	}
	for i, want := range expected {
		if order[i] != want {
			t.Errorf("position %d: expected %q, got %q", i, want, order[i])
		}
	}
}

func TestChainEmpty(t *testing.T) {
	called := false
	final := func(ctx context.Context, req *Request) (*Response, error) {
		called = true
		return &Response{}, nil
	}

	composed := Chain()(final)
	_, err := composed(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected final handler to be called")
	}
}

func TestChainSingle(t *testing.T) {
	var order []string

	m := func(next Handler) Handler {
		return func(ctx context.Context, req *Request) (*Response, error) {
			order = append(order, "m-before")
			resp, err := next(ctx, req)
			order = append(order, "m-after")
			return resp, err
		}
	}

	final := func(ctx context.Context, req *Request) (*Response, error) {
		order = append(order, "final")
		return &Response{}, nil
	}

	composed := Chain(m)(final)
	_, err := composed(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"m-before", "final", "m-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d events, got %d: %v", len(expected), len(order), order)
	}
	for i, want := range expected {
		if order[i] != want {
			t.Errorf("position %d: expected %q, got %q", i, want, order[i])
		}
	}
}

func TestChainContextPropagation(t *testing.T) {
	type ctxKey struct{}

	m := func(next Handler) Handler {
		return func(ctx context.Context, req *Request) (*Response, error) {
			ctx = context.WithValue(ctx, ctxKey{}, "from-middleware")
			return next(ctx, req)
		}
	}

	final := func(ctx context.Context, req *Request) (*Response, error) {
		v := ctx.Value(ctxKey{})
		if v != "from-middleware" {
			t.Errorf("expected context value not propagated: got %v", v)
		}
		return &Response{}, nil
	}

	composed := Chain(m)(final)
	_, err := composed(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

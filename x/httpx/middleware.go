package httpx

// Chain composes middlewares from left to right: Chain(m1, m2)(final)
// produces m1(m2(final)). Execution order is m1-before → m2-before →
// final → m2-after → m1-after.
func Chain(middlewares ...Middleware) Middleware {
	return func(next Handler) Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

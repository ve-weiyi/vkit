package httpx

import "context"

// hookMiddleware 返回一个在最外层包装的中间件，
// 在请求发出前调用 OnRequest，在响应返回或出错后调用 OnResponse/OnError。
func hookMiddleware(hooks *Hooks) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req *Request) (*Response, error) {
			if hooks.OnRequest != nil {
				hooks.OnRequest(req)
			}
			resp, err := next(ctx, req)
			if err != nil && hooks.OnError != nil {
				hooks.OnError(req, err)
			}
			if resp != nil && hooks.OnResponse != nil {
				hooks.OnResponse(resp)
			}
			return resp, err
		}
	}
}

// WithHooks 设置请求生命周期回调。存储的是传入结构体的拷贝，
// 避免调用方后续修改影响 Client 行为。
func WithHooks(h *Hooks) Option {
	hooksCopy := *h
	return func(c *config) {
		c.client.hooks = &hooksCopy
	}
}

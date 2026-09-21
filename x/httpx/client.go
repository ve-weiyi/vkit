package httpx

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// === Construction ===

// defaultMaxBodyLogLen is the default max length for body logging/error messages.
const defaultMaxBodyLogLen = 4096

// Client HTTP 客户端。
type Client struct {
	httpClient *http.Client
	cfg        *config
}

// New 创建 Client，应用 Client 级 Option。
func New(opts ...Option) *Client {
	cfg := &config{}
	cfg.client.transport = DefaultTransport()
	cfg.client.timeout = 30 * time.Second
	for _, opt := range opts {
		opt(cfg)
	}

	return &Client{
		httpClient: &http.Client{
			Transport: buildTransport(cfg),
			Timeout:   cfg.client.timeout,
		},
		cfg: cfg,
	}
}

// buildTransport 构建最终的 http.RoundTripper 链路：
//
//	DefaultTransport / WithTransport → WithRoundTripper → InsecureSkipVerify → LoggingTransport
//
// InsecureSkipVerify 在最终 Transport 上生效，避免被 WithRoundTripper 覆盖。
func buildTransport(cfg *config) http.RoundTripper {
	var t http.RoundTripper = cfg.client.transport

	if cfg.client.roundTripper != nil {
		t = cfg.client.roundTripper
	}

	if cfg.client.insecureSkipVerify {
		if tr, ok := t.(*http.Transport); ok {
			if tr.TLSClientConfig == nil {
				tr.TLSClientConfig = &tls.Config{}
			}
			tr.TLSClientConfig.InsecureSkipVerify = true
		}
	}

	if cfg.client.loggingTransport != nil {
		t = newLoggingTransport(t, cfg.client.loggingTransport)
	}

	return t
}

// === Execution Pipeline ===

func (cl *Client) execute(ctx context.Context, opts ...Option) (*Response, error) {
	cfg := cl.cloneConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// 请求级 WithTimeout 必须在这里落到 ctx 上：http.Client.Timeout 在 New 时已定型，
	// 之后改 cfg 不会生效（旧实现是静默无效）
	if t := cfg.client.timeout; t > 0 && t != cl.cfg.client.timeout {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t)
		defer cancel()
	}

	if err := cl.serializeBody(cfg); err != nil {
		return nil, err
	}

	httpReq, err := cl.newHTTPRequest(ctx, cfg)
	if err != nil {
		return nil, err
	}

	req := &Request{
		Request:   httpReq,
		Attempt:   1,
		StartedAt: time.Now(),
		Stream:    cfg.req.stream,
	}

	return cl.buildChain(cfg)(ctx, req)
}

// serializeBody 序列化请求体，将结果写入 cfg.req.bodyData 和 cfg.req.multipartCT。
func (cl *Client) serializeBody(cfg *config) error {
	if cfg.req.json != nil {
		data, err := json.Marshal(cfg.req.json)
		if err != nil {
			return fmt.Errorf("httpx: marshal json: %w", err)
		}
		cfg.req.bodyData = data
		return nil
	}
	if cfg.req.form != nil {
		values := url.Values{}
		for k, v := range cfg.req.form {
			values.Set(k, v)
		}
		cfg.req.bodyData = []byte(values.Encode())
		return nil
	}
	if cfg.req.multipart != nil {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		for key, val := range cfg.req.multipart.fields {
			if err := w.WriteField(key, val); err != nil {
				return fmt.Errorf("httpx: write multipart field: %w", err)
			}
		}
		part, err := w.CreateFormFile(cfg.req.multipart.fileField, cfg.req.multipart.filename)
		if err != nil {
			return fmt.Errorf("httpx: create form file: %w", err)
		}
		if _, err := io.Copy(part, cfg.req.multipart.reader); err != nil {
			return fmt.Errorf("httpx: copy file content: %w", err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("httpx: close multipart writer: %w", err)
		}
		cfg.req.bodyData = buf.Bytes()
		cfg.req.multipartCT = w.FormDataContentType()
		return nil
	}
	return nil
}

// newHTTPRequest 基于已序列化的配置创建 *http.Request。
func (cl *Client) newHTTPRequest(ctx context.Context, cfg *config) (*http.Request, error) {
	reqURL, err := buildURL(cfg)
	if err != nil {
		return nil, err
	}

	var body io.Reader
	if cfg.req.bodyData != nil {
		body = bytes.NewReader(cfg.req.bodyData)
	}

	httpReq, err := http.NewRequestWithContext(ctx, cfg.req.method, reqURL, body)
	if err != nil {
		return nil, err
	}

	// Set GetBody for retry replay (skip for stream mode since body can't be replayed)
	if cfg.req.bodyData != nil && !cfg.req.stream {
		httpReq.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(cfg.req.bodyData)), nil
		}
	}

	cl.setHeaders(httpReq, cfg)

	return httpReq, nil
}

// buildChain 构建中间件链，Hooks 位于最外层。
func (cl *Client) buildChain(cfg *config) Handler {
	mws := cfg.client.middlewares
	if cfg.client.hooks != nil {
		mws = append([]Middleware{hookMiddleware(cfg.client.hooks)}, mws...)
	}
	return Chain(mws...)(cl.coreHandler(cfg))
}

func (cl *Client) coreHandler(cfg *config) Handler {
	return func(ctx context.Context, req *Request) (*Response, error) {
		start := time.Now()
		httpResp, err := cl.httpClient.Do(req.Request)
		if err != nil {
			return nil, err
		}
		duration := time.Since(start)

		resp := &Response{
			Response: httpResp,
			Request:  req,
			Duration: duration,
			Attempt:  req.Attempt,
		}

		// 非 stream 模式：读取完整 body
		if !cfg.req.stream {
			bodyBytes, readErr := io.ReadAll(httpResp.Body)
			httpResp.Body.Close()
			if readErr != nil {
				return nil, readErr
			}
			resp.Body = bodyBytes
			httpResp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// 非 2xx 返回 Error
		if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
			if cfg.req.stream {
				httpResp.Body.Close()
			}
			var body string
			if !cfg.req.stream {
				body = truncateBody(string(resp.Body), defaultMaxBodyLogLen)
			}
			return nil, &Error{
				StatusCode: httpResp.StatusCode,
				Body:       body,
			}
		}

		return resp, nil
	}
}

// === Public Methods ===

// Get 发送 GET 请求。
func (cl *Client) Get(ctx context.Context, path string, opts ...Option) (*Response, error) {
	return cl.execute(ctx, append(opts, withMethod(http.MethodGet), withPath(path))...)
}

// Post 发送 POST 请求。
func (cl *Client) Post(ctx context.Context, path string, opts ...Option) (*Response, error) {
	return cl.execute(ctx, append(opts, withMethod(http.MethodPost), withPath(path))...)
}

// Put 发送 PUT 请求。
func (cl *Client) Put(ctx context.Context, path string, opts ...Option) (*Response, error) {
	return cl.execute(ctx, append(opts, withMethod(http.MethodPut), withPath(path))...)
}

// Delete 发送 DELETE 请求。
func (cl *Client) Delete(ctx context.Context, path string, opts ...Option) (*Response, error) {
	return cl.execute(ctx, append(opts, withMethod(http.MethodDelete), withPath(path))...)
}

// Patch 发送 PATCH 请求。
func (cl *Client) Patch(ctx context.Context, path string, opts ...Option) (*Response, error) {
	return cl.execute(ctx, append(opts, withMethod(http.MethodPatch), withPath(path))...)
}

// Do 发送任意 HTTP method 的请求。method 不区分大小写。
func (cl *Client) Do(ctx context.Context, method, path string, opts ...Option) (*Response, error) {
	return cl.execute(ctx, append(opts, withMethod(method), withPath(path))...)
}

// === Internal Helpers ===

func withMethod(method string) Option {
	return func(c *config) { c.req.method = method }
}

func withPath(path string) Option {
	return func(c *config) { c.req.url = path }
}

// cloneConfig 复制 Client 级配置创建请求级配置。
// headers map 必须深拷贝：请求级 Option（WithHeaders / WithUserAgent）会写它，
// 浅拷贝会让写入落到 client 共享的 map 上——既污染该 client 后续所有请求，也是数据竞争。
// middlewares / hooks 等其余引用类型仍共享只读。
func (cl *Client) cloneConfig() *config {
	clientCfg := cl.cfg.client
	clientCfg.headers = maps.Clone(cl.cfg.client.headers)
	return &config{
		client: clientCfg,
	}
}

func buildURL(cfg *config) (string, error) {
	raw := cfg.req.url
	// 若 url 已是绝对地址（自带 scheme），不拼接 baseURL
	if !strings.Contains(cfg.req.url, "://") {
		raw = cfg.client.baseURL + "/" + strings.TrimLeft(cfg.req.url, "/")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("httpx: parse url: %w", err)
	}
	if len(cfg.req.query) > 0 {
		q := u.Query()
		for k, v := range cfg.req.query {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

func (cl *Client) setHeaders(req *http.Request, cfg *config) {
	for k, v := range cfg.client.headers {
		req.Header.Set(k, v)
	}
	for k, v := range cfg.req.headers {
		req.Header.Set(k, v)
	}
	if cfg.req.json != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cfg.req.form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if cfg.req.multipartCT != "" {
		req.Header.Set("Content-Type", cfg.req.multipartCT)
	}
}

// truncateBody 安全截断字符串到 maxLen，确保不切断 UTF-8 多字节字符。
func truncateBody(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	for maxLen > 0 && s[maxLen-1]&0xC0 == 0x80 {
		maxLen--
	}
	return s[:maxLen] + "... (truncated)"
}

// === Client Options ===

// WithBaseURL 设置基础 URL。
func WithBaseURL(baseURL string) Option {
	return func(c *config) {
		c.client.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithTimeout 设置请求超时。
func WithTimeout(timeout time.Duration) Option {
	return func(c *config) {
		c.client.timeout = timeout
	}
}

// WithHeaders 设置请求头。
// 作为 Client Option 传入时是所有请求的默认头；作为请求 Option 传入时只作用于该次请求。
func WithHeaders(headers map[string]string) Option {
	return func(c *config) {
		if c.client.headers == nil {
			c.client.headers = make(map[string]string)
		}
		for k, v := range headers {
			c.client.headers[k] = v
		}
	}
}

// WithMiddleware 注册中间件。支持同时注册多个中间件。
func WithMiddleware(mws ...Middleware) Option {
	return func(c *config) {
		c.client.middlewares = append(c.client.middlewares, mws...)
	}
}

// WithRetry 启用自动重试。
func WithRetry(cfg RetryConfig) Option {
	return func(c *config) {
		c.client.middlewares = append(c.client.middlewares, RetryMiddleware(cfg))
	}
}

// WithUserAgent 设置默认 User-Agent 请求头。
func WithUserAgent(ua string) Option {
	return func(c *config) {
		if c.client.headers == nil {
			c.client.headers = make(map[string]string)
		}
		c.client.headers["User-Agent"] = ua
	}
}

// WithRoundTripper 设置自定义 RoundTripper，覆盖默认 Transport。
func WithRoundTripper(rt http.RoundTripper) Option {
	return func(c *config) {
		c.client.roundTripper = rt
	}
}

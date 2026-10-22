package httpx

import (
	"context"
	"io"
	"net/http"
	"time"
)

// Handler is the function signature for HTTP handlers in the middleware chain.
type Handler func(ctx context.Context, req *Request) (*Response, error)

// Middleware wraps a Handler to add cross-cutting concerns.
type Middleware func(Handler) Handler

// Hooks 请求生命周期回调。
type Hooks struct {
	OnRequest  func(req *Request)
	OnResponse func(resp *Response)
	OnError    func(req *Request, err error)
}

// multipartConfig multipart 表单配置。
type multipartConfig struct {
	fields    map[string]string
	fileField string
	filename  string
	reader    io.Reader
}

// RetryConfig controls automatic retry behavior.
type RetryConfig struct {
	MaxAttempts int
	Delay       time.Duration
}

// Request 包装 http.Request，携带中间件元数据。
type Request struct {
	*http.Request
	Attempt   int       // 当前第几次尝试（从 1 开始）
	StartedAt time.Time // 首次请求发起时间
	Stream    bool      // true 表示流式模式，跳过重试
}

// Response 包装 http.Response。
type Response struct {
	*http.Response
	Body     []byte // 普通模式：已读取的响应体
	Request  *Request
	Duration time.Duration
	Attempt  int
}

// clientConfig Client 创建后不可变（immutable after New），所有请求共享只读。
// 含引用类型（headers map、middlewares slice），值拷贝后底层数据共享，禁止修改。
type clientConfig struct {
	baseURL            string
	timeout            time.Duration
	transport          *http.Transport
	headers            map[string]string
	middlewares        []Middleware
	hooks              *Hooks
	insecureSkipVerify bool
	roundTripper       http.RoundTripper
	loggingTransport   io.Writer
}

// requestConfig 每次请求独立创建，零值即可使用。
type requestConfig struct {
	method      string
	url         string
	query       map[string]string
	headers     map[string]string
	form        map[string]string
	json        interface{}
	multipart   *multipartConfig
	stream      bool
	bodyData    []byte
	multipartCT string
}

// config 内部配置，Option 函数修改此结构体。
type config struct {
	client clientConfig
	req    requestConfig
}

// Option 函数式配置。
type Option func(*config)

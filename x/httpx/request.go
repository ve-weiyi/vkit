package httpx

import "io"

// WithQuery sets a single query parameter. Multiple calls are additive.
func WithQuery(key, value string) Option {
	return func(c *config) {
		if c.req.query == nil {
			c.req.query = make(map[string]string)
		}
		c.req.query[key] = value
	}
}

// WithHeader sets a single request header. Multiple calls are additive.
func WithHeader(key, value string) Option {
	return func(c *config) {
		if c.req.headers == nil {
			c.req.headers = make(map[string]string)
		}
		c.req.headers[key] = value
	}
}

// WithJSON sets the JSON request body. Automatically sets Content-Type to
// application/json when the request is sent, overriding any Content-Type
// header set via WithHeader or client-level WithHeaders.
func WithJSON(v interface{}) Option {
	return func(c *config) {
		c.req.json = v
	}
}

// WithForm sets the form-encoded request body. Automatically sets Content-Type
// to application/x-www-form-urlencoded when the request is sent, overriding
// any Content-Type header set via WithHeader or client-level WithHeaders.
func WithForm(values map[string]string) Option {
	return func(c *config) {
		c.req.form = values
	}
}

// WithBody sets the raw request body. The caller is responsible for
// setting the appropriate Content-Type header via WithHeader.
func WithBody(b []byte) Option {
	return func(c *config) {
		c.req.bodyData = b
	}
}

// WithStream enables streaming mode. When streaming, the response body
// is not pre-read into memory; the caller is responsible for closing it.
func WithStream() Option {
	return func(c *config) {
		c.req.stream = true
	}
}

// WithMultipart constructs a multipart/form-data request body with the given
// form fields and a single file. Automatically sets Content-Type (including
// boundary) when the request is sent, overriding any Content-Type header set
// via WithHeader or client-level WithHeaders.
// The reader will be consumed when the request is built, so the entire body
// is buffered in memory to support retry.
func WithMultipart(fields map[string]string, fileField, filename string, reader io.Reader) Option {
	return func(c *config) {
		c.req.multipart = &multipartConfig{
			fields:    fields,
			fileField: fileField,
			filename:  filename,
			reader:    reader,
		}
	}
}

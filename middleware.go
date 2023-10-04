package goyavetrace

import "goyave.dev/goyave/v4"

// Middleware substitutes the original response writer with a trace writer wrapper.
type Middleware struct {

	// SpanOption if not nil, this function is executed before finishing the span
	// associated with the request. This can be used to add custom tags to the span.
	SpanOption SpanOption
}

// Handle substitutes the original response writer with a trace writer wrapper.
func (m Middleware) Handle(next goyave.Handler) goyave.Handler {
	return func(resp *goyave.Response, req *goyave.Request) {
		traceWriter := NewWriter(resp, req, m.SpanOption)
		resp.SetWriter(traceWriter)
		next(resp, req)
	}
}

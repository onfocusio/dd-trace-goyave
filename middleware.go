package goyavetrace

import "goyave.dev/goyave/v4"

// Middleware substitutes the original response writer with a trace writer wrapper.
type Middleware struct{}

// Handle substitutes the original response writer with a trace writer wrapper.
func (Middleware) Handle(next goyave.Handler) goyave.Handler {
	return func(resp *goyave.Response, req *goyave.Request) {
		traceWriter := NewWriter(resp, req)
		resp.SetWriter(traceWriter)
		next(resp, req)
	}
}

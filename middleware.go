package goyavetrace

import "goyave.dev/goyave/v4"

// Middleware replaces the response writer with a trace writer.
type Middleware struct{}

// Handle replaces the response writer with a trace writer.
func (Middleware) Handle(next goyave.Handler) goyave.Handler {
	return func(resp *goyave.Response, req *goyave.Request) {
		traceWriter := NewWriter(resp, req)
		resp.SetWriter(traceWriter)
		next(resp, req)
	}
}

package goyavetrace

import (
	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"goyave.dev/goyave/v5"
)

// Middleware substitutes the original response writer with a trace writer wrapper.
type Middleware struct {
	goyave.Component
	cfg Config

	// SpanOptions these functions are executed before finishing the span
	// associated with the request. This can be used to add custom tags to the span.
	SpanOptions []SpanOption
}

// NewMiddleware create a new trace middleware.
func NewMiddleware(cfg Config, spanOptions ...SpanOption) *Middleware {
	return &Middleware{
		cfg:         cfg,
		SpanOptions: spanOptions,
	}
}

// Handle substitutes the original response writer with a trace writer wrapper.
func (m Middleware) Handle(next goyave.Handler) goyave.Handler {
	return func(resp *goyave.Response, req *goyave.Request) {
		traceWriter := NewWriter(resp, req, m.cfg, m.SpanOptions...)
		resp.SetWriter(traceWriter)

		ctx := tracer.ContextWithSpan(req.Context(), traceWriter.span)
		req.WithContext(ctx)
		next(resp, req)
	}
}

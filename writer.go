package goyavetrace

import (
	"io"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/ext"
	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"goyave.dev/goyave/v5"
	"goyave.dev/goyave/v5/util/errors"
)

// Writer doesn't affect the response but captures information about the request
// and the response to convert it as tags for a datadog span. The span is finished and
// reported when `Close()` is called.
type Writer struct {
	writer      io.Writer
	request     *goyave.Request
	response    *goyave.Response
	span        *tracer.Span
	spanOptions []SpanOption
}

// NewWriter creates a new writer meant for use in a single response.
// Starts the span right away with the common options (service name, uri, method, route name, span kind and type).
//
// The given `SpanOption` is executed before ending the span (at the end of the request life-cycle) and can be used to add
// tags to the span. Can be `nil`.
func NewWriter(response *goyave.Response, request *goyave.Request, cfg Config, spanOptions ...SpanOption) *Writer {
	spanOpts := []tracer.StartSpanOption{
		tracer.ServiceName(cfg.Service),
		tracer.Tag(ext.Environment, cfg.Env),
		tracer.Tag(ext.SpanType, ext.SpanTypeWeb),
		tracer.Tag(ext.HTTPURL, request.Request().RequestURI),
		tracer.Tag(ext.HTTPMethod, request.Method()),
		tracer.Tag(ext.HTTPRoute, request.Route.GetName()),
		tracer.Tag(ext.HTTPUserAgent, request.UserAgent()),
		tracer.Tag(ext.SpanKind, ext.SpanKindServer),
		tracer.Tag(ext.Component, componentName),
		func(cfg *tracer.StartSpanConfig) {
			if spanctx, err := tracer.Extract(tracer.HTTPHeadersCarrier(request.Header())); err == nil {
				cfg.Parent = spanctx
			}
		},
	}

	return &Writer{
		writer:      response.Writer(),
		request:     request,
		response:    response,
		span:        tracer.StartSpan("web.request", spanOpts...),
		spanOptions: spanOptions,
	}
}

// PreWrite calls PreWrite on the
// child writer if it implements PreWriter.
func (w *Writer) PreWrite(b []byte) {
	if pr, ok := w.writer.(goyave.PreWriter); ok {
		pr.PreWrite(b)
	}
}

func (w *Writer) Write(b []byte) (int, error) {
	c, err := w.writer.Write(b)
	return c, errors.New(err)
}

// Close the underlying writer, adds the status, error, and user tags to the span
// and finishes it.
func (w *Writer) Close() error {
	w.span.SetTag(ext.HTTPCode, w.response.GetStatus())
	if err := w.response.GetError(); err != nil {
		w.span.SetTag(ext.Error, err)
		w.span.SetTag(ext.ErrorStack, err.StackFrames().String())
	}

	if u, ok := w.request.User.(DatadogUserConverter); ok {
		w.span.SetTag(TagUser, u.ToDatadogUser().String())
	}

	for _, opt := range w.spanOptions {
		opt(w.span, w.response, w.request)
	}

	w.span.Finish()

	if wr, ok := w.writer.(io.Closer); ok {
		return errors.New(wr.Close())
	}
	return nil
}

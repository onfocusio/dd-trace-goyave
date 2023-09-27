package goyavetrace

import (
	"io"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"goyave.dev/goyave/v4"
	"goyave.dev/goyave/v4/config"
)

// Writer doesn't affect the response but captures information about the request
// and the response to convert it as tags for a datadog span. The span is finished and
// reported when `Close()` is called.
type Writer struct {
	writer   io.Writer
	request  *goyave.Request
	response *goyave.Response
	span     tracer.Span
}

// NewWriter creates a new writer meant for use in a single response.
// Starts the span right away with the common options (service name, uri, method, route name, span kind and type).
func NewWriter(response *goyave.Response, request *goyave.Request) *Writer {
	spanOpts := []tracer.StartSpanOption{
		tracer.ServiceName(config.GetString("app.datadog.service")),
		tracer.Tag(ext.Environment, config.GetString("app.environment")),
		tracer.Tag(ext.SpanType, ext.SpanTypeWeb),
		tracer.Tag(ext.HTTPURL, request.Request().RequestURI),
		tracer.Tag(ext.HTTPMethod, request.Method()),
		tracer.Tag(ext.HTTPRoute, request.Route().GetName()),
		tracer.Tag(ext.HTTPUserAgent, request.UserAgent()),
		tracer.Tag(ext.SpanKind, ext.SpanKindServer),
		tracer.Tag(ext.Component, "goyave.dev/goyave"),
		func(cfg *ddtrace.StartSpanConfig) {
			if spanctx, err := tracer.Extract(tracer.HTTPHeadersCarrier(request.Header())); err == nil {
				cfg.Parent = spanctx
			}
		},
	}

	return &Writer{
		writer:   response.Writer(),
		request:  request,
		response: response,
		span:     tracer.StartSpan("web.request", spanOpts...),
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
	return w.writer.Write(b)
}

// Close the underlying writer, adds the status, error, and user tags to the span
// and finishes it.
func (w *Writer) Close() error {
	if wr, ok := w.writer.(io.Closer); ok {
		return wr.Close()
	}

	w.span.SetTag(ext.HTTPCode, w.response.GetStatus())
	if err := w.response.GetError(); err != nil {
		w.span.SetTag(ext.Error, err)
		w.span.SetTag(ext.ErrorStack, w.response.GetStacktrace())
	}

	if u, ok := w.request.User.(DatadogUserConverter); ok {
		w.span.SetTag(TagUser, u.ToDatadogUser().String())
	}

	w.span.Finish()

	return nil
}

package goyavetrace

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/ext"
	"github.com/DataDog/dd-trace-go/v2/ddtrace/mocktracer"
	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"goyave.dev/goyave/v5"
	"goyave.dev/goyave/v5/config"
	"goyave.dev/goyave/v5/slog"
	"goyave.dev/goyave/v5/util/testutil"
)

type testUser struct {
	Name  string
	Email string
	ID    int64
}

func (u testUser) ToDatadogUser() DatadogUser {
	return DatadogUser(u)
}

func TestWriter(t *testing.T) {
	mt := mocktracer.Start()
	defer mt.Stop()

	middleware := NewMiddleware(Config{
		AgentAddr: "localhost:8126",
		Env:       "test",
		Service:   "service-name",
	})

	server := testutil.NewTestServerWithOptions(t, goyave.Options{Config: config.LoadDefault()})

	request := server.NewTestRequest(http.MethodGet, "/test/param-value", nil)
	request.Header().Set(tracer.DefaultParentIDHeader, "1234567")
	request.Header().Set(tracer.DefaultTraceIDHeader, "7654321")
	request.Header().Set(tracer.DefaultPriorityHeader, "1")
	request.Route = server.Router().Get("/test/{param}", nil).Name("test-route")

	result := server.TestMiddleware(middleware, request, func(resp *goyave.Response, req *goyave.Request) {
		req.User = &testUser{ID: 1, Name: "test", Email: "test@example.org"}

		writer := resp.Writer()
		traceWriter, ok := writer.(*Writer)
		if !assert.True(t, ok) {
			return
		}
		assert.NotNil(t, traceWriter.span)
		assert.Equal(t, resp, traceWriter.response)
		assert.Equal(t, req, traceWriter.request)

		resp.String(http.StatusForbidden, "forbidden message")
	})

	body, err := io.ReadAll(result.Body)
	assert.NoError(t, result.Body.Close())
	assert.NoError(t, err)

	assert.Equal(t, "forbidden message", string(body))
	assert.Equal(t, http.StatusForbidden, result.StatusCode)

	spans := mt.FinishedSpans()
	require.Len(t, spans, 1)

	span := spans[0]
	assert.Equal(t, "web.request", span.OperationName())
	assert.Equal(t, "goyave.dev/goyave", span.Tag(ext.Component))
	assert.Equal(t, "service-name", span.Tag(ext.ServiceName))
	assert.Equal(t, "test", span.Tag(ext.Environment))
	assert.Equal(t, ext.SpanTypeWeb, span.Tag(ext.SpanType))
	assert.Equal(t, ext.SpanKindServer, span.Tag(ext.SpanKind))
	assert.Equal(t, "/test/param-value", span.Tag(ext.HTTPURL))
	assert.Equal(t, http.MethodGet, span.Tag(ext.HTTPMethod))
	assert.Equal(t, "test-route", span.Tag(ext.HTTPRoute))
	assert.InDelta(t, float64(http.StatusForbidden), span.Tag(ext.HTTPCode), 0)
	assert.Equal(t, `{"Name":"test","Email":"test@example.org","ID":1}`, span.Tag(TagUser))
	assert.Equal(t, uint64(1234567), span.ParentID())
	assert.Equal(t, uint64(7654321), span.TraceID())
}

func TestWriterWithError(t *testing.T) {
	mt := mocktracer.Start()
	defer mt.Stop()

	middleware := NewMiddleware(Config{
		AgentAddr: "localhost:8126",
		Env:       "test",
		Service:   "service-name",
	})

	server := testutil.NewTestServerWithOptions(t, goyave.Options{
		Config: config.LoadDefault(),
		Logger: slog.New(slog.NewHandler(false, &bytes.Buffer{})),
	})

	request := server.NewTestRequest(http.MethodGet, "/test", nil)
	request.Route = server.Router().Get("/test/{param}", nil).Name("test-route")
	result := server.TestMiddleware(middleware, request, func(_ *goyave.Response, _ *goyave.Request) {
		panic("custom error")
	})

	body, err := io.ReadAll(result.Body)
	assert.NoError(t, result.Body.Close())
	assert.NoError(t, err)

	assert.Equal(t, "{\"error\":\"custom error\"}\n", string(body))
	assert.Equal(t, http.StatusInternalServerError, result.StatusCode)

	spans := mt.FinishedSpans()
	require.Len(t, spans, 1)

	span := spans[0]
	assert.Equal(t, "web.request", span.OperationName())
	assert.Equal(t, "goyave.dev/goyave", span.Tag(ext.Component))
	assert.Equal(t, "service-name", span.Tag(ext.ServiceName))
	assert.Equal(t, "test", span.Tag(ext.Environment))
	assert.Equal(t, ext.SpanTypeWeb, span.Tag(ext.SpanType))
	assert.Equal(t, ext.SpanKindServer, span.Tag(ext.SpanKind))
	assert.Equal(t, "/test", span.Tag(ext.HTTPURL))
	assert.Equal(t, http.MethodGet, span.Tag(ext.HTTPMethod))
	assert.Equal(t, "test-route", span.Tag(ext.HTTPRoute))
	assert.InDelta(t, float64(http.StatusInternalServerError), span.Tag(ext.HTTPCode), 0)
	assert.Equal(t, "custom error", span.Tag(ext.ErrorMsg))
	assert.Equal(t, "*errors.Error", span.Tag(ext.ErrorType))
	assert.NotEmpty(t, span.Tag(ext.ErrorStack))
	// In actual implementation (not mock), the ext.ErrorMsg and ext.ErrorType are added
}

func TestSpanContext(t *testing.T) {
	mt := mocktracer.Start()
	defer mt.Stop()

	middleware := NewMiddleware(Config{
		AgentAddr: "localhost:8126",
		Env:       "test",
		Service:   "service-name",
	}, func(s *tracer.Span, resp *goyave.Response, req *goyave.Request) {
		assert.NotNil(t, resp)
		assert.NotNil(t, req)
		s.SetTag(ext.ManualKeep, true)
	})

	server := testutil.NewTestServerWithOptions(t, goyave.Options{Config: config.LoadDefault()})

	request := server.NewTestRequest(http.MethodGet, "/test", nil)
	request.Route = server.Router().Get("/test/{param}", nil).Name("test-route")

	result := server.TestMiddleware(middleware, request, func(resp *goyave.Response, req *goyave.Request) {
		ctx := req.Context()

		span, _ := tracer.StartSpanFromContext(ctx, "test-span")
		span.Finish()

		resp.String(http.StatusNoContent, "")
	})

	assert.NoError(t, result.Body.Close())
	assert.Equal(t, http.StatusNoContent, result.StatusCode)

	spans := mt.FinishedSpans()
	require.Len(t, spans, 2)

	{
		span := spans[0]
		assert.Equal(t, "test-span", span.OperationName())
		assert.NotZero(t, span.ParentID())
	}

	{
		span := spans[1]
		assert.Equal(t, "web.request", span.OperationName())
		assert.Equal(t, "goyave.dev/goyave", span.Tag(ext.Component))
		assert.Equal(t, "service-name", span.Tag(ext.ServiceName))
		assert.Equal(t, "test", span.Tag(ext.Environment))
		assert.Equal(t, ext.SpanTypeWeb, span.Tag(ext.SpanType))
		assert.Equal(t, ext.SpanKindServer, span.Tag(ext.SpanKind))
		assert.Equal(t, "/test", span.Tag(ext.HTTPURL))
		assert.Equal(t, http.MethodGet, span.Tag(ext.HTTPMethod))
		assert.Equal(t, "test-route", span.Tag(ext.HTTPRoute))
		assert.InDelta(t, float64(ext.PriorityUserKeep), span.Tag("_sampling_priority_v1"), 0)
	}
}

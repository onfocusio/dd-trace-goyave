package goyavetrace

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/mocktracer"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"goyave.dev/goyave/v4"
	"goyave.dev/goyave/v4/config"
)

type testUser struct {
	Name  string
	Email string
	ID    int64
}

func (u testUser) ToDatadogUser() DatadogUser {
	return DatadogUser(u)
}

type MiddlewareTestSuite struct {
	goyave.TestSuite
}

func (suite *MiddlewareTestSuite) TestWriter() {
	err := config.LoadJSON(`{"app": {"environment": "test", "datadog": {"service": "service-name"}}}`)
	if !suite.NoError(err) {
		return
	}

	mt := mocktracer.Start()
	defer mt.Stop()
	suite.RunServer(func(r *goyave.Router) {
		r.GlobalMiddleware((&Middleware{
			SpanOption: func(s tracer.Span, resp *goyave.Response, req *goyave.Request) {
				suite.NotNil(resp)
				suite.NotNil(req)
				s.SetTag(ext.ManualKeep, true)
			},
		}).Handle)
		r.Get("/test/{param}", func(response *goyave.Response, request *goyave.Request) {
			request.User = &testUser{ID: 1, Name: "test", Email: "test@example.org"}
			suite.NoError(response.String(http.StatusForbidden, "forbidden message"))

			writer := response.Writer()
			traceWriter, ok := writer.(*Writer)
			if !suite.True(ok) {
				return
			}

			suite.NotNil(traceWriter.span)
			suite.Equal(response, traceWriter.response)
			suite.Equal(request, traceWriter.request)
		}).Name("test-route")
	}, func() {
		headers := map[string]string{
			tracer.DefaultParentIDHeader: "1234567",
			tracer.DefaultTraceIDHeader:  "7654321",
			tracer.DefaultPriorityHeader: "1",
		}
		result, err := suite.Get("/test/param-value", headers)

		if !suite.NoError(err) {
			return
		}
		body := suite.GetBody(result)
		suite.Equal(string(body), "forbidden message")
		suite.Equal(http.StatusForbidden, result.StatusCode)

		spans := mt.FinishedSpans()
		if !suite.Len(spans, 1) {
			return
		}

		span := spans[0]
		suite.Equal("web.request", span.OperationName())
		suite.Equal("goyave.dev/goyave", span.Tag(ext.Component))
		suite.Equal("service-name", span.Tag(ext.ServiceName))
		suite.Equal("test", span.Tag(ext.Environment))
		suite.Equal(ext.SpanTypeWeb, span.Tag(ext.SpanType))
		suite.Equal(ext.SpanKindServer, span.Tag(ext.SpanKind))
		suite.Equal("/test/param-value", span.Tag(ext.HTTPURL))
		suite.Equal(http.MethodGet, span.Tag(ext.HTTPMethod))
		suite.Equal("test-route", span.Tag(ext.HTTPRoute))
		suite.Equal(http.StatusForbidden, span.Tag(ext.HTTPCode))
		suite.Equal(true, span.Tag(ext.ManualKeep))
		suite.Equal(`{"Name":"test","Email":"test@example.org","ID":1}`, span.Tag(TagUser))
		suite.Equal(uint64(1234567), span.ParentID())
		suite.Equal(uint64(7654321), span.TraceID())
	})
}

func (suite *MiddlewareTestSuite) TestWriterWithError() {
	err := config.LoadJSON(`{"app": {"environment": "test", "datadog": {"service": "service-name"}}}`)
	if !suite.NoError(err) {
		return
	}

	mt := mocktracer.Start()
	defer mt.Stop()
	suite.RunServer(func(r *goyave.Router) {
		r.GlobalMiddleware((&Middleware{}).Handle)
		r.Get("/test", func(response *goyave.Response, request *goyave.Request) {
			panic(fmt.Errorf("custom error"))
		}).Name("test-route")
	}, func() {
		result, err := suite.Get("/test", nil)

		if !suite.NoError(err) {
			return
		}
		body := suite.GetBody(result)
		suite.Equal(string(body), "{\"error\":\"custom error\"}\n")
		suite.Equal(http.StatusInternalServerError, result.StatusCode)

		spans := mt.FinishedSpans()
		if !suite.Len(spans, 1) {
			return
		}

		span := spans[0]
		suite.Equal("web.request", span.OperationName())
		suite.Equal("goyave.dev/goyave", span.Tag(ext.Component))
		suite.Equal("service-name", span.Tag(ext.ServiceName))
		suite.Equal("test", span.Tag(ext.Environment))
		suite.Equal(ext.SpanTypeWeb, span.Tag(ext.SpanType))
		suite.Equal(ext.SpanKindServer, span.Tag(ext.SpanKind))
		suite.Equal("/test", span.Tag(ext.HTTPURL))
		suite.Equal(http.MethodGet, span.Tag(ext.HTTPMethod))
		suite.Equal("test-route", span.Tag(ext.HTTPRoute))
		suite.Equal(http.StatusInternalServerError, span.Tag(ext.HTTPCode))
		suite.Equal(fmt.Errorf("custom error"), span.Tag(ext.Error))
		suite.NotEmpty(span.Tag(ext.ErrorStack))
		// In actual implementation (not mock), the ext.ErrorMsg and ext.ErrorType are added
	})
}

func (suite *MiddlewareTestSuite) TestSpanContext() {
	err := config.LoadJSON(`{"app": {"environment": "test", "datadog": {"service": "service-name"}}}`)
	if !suite.NoError(err) {
		return
	}

	mt := mocktracer.Start()
	defer mt.Stop()
	suite.RunServer(func(r *goyave.Router) {
		r.GlobalMiddleware((&Middleware{
			SpanOption: func(s tracer.Span, resp *goyave.Response, req *goyave.Request) {
				suite.NotNil(resp)
				suite.NotNil(req)
				s.SetTag(ext.ManualKeep, true)
			},
		}).Handle)
		r.Get("/test", func(response *goyave.Response, request *goyave.Request) {
			ctx := request.Request().Context()

			span, _ := tracer.StartSpanFromContext(ctx, "test-span")
			span.Finish()

			suite.NoError(response.String(http.StatusNoContent, ""))
		}).Name("test-route")
	}, func() {
		result, err := suite.Get("/test", nil)

		if !suite.NoError(err) {
			return
		}
		suite.Equal(http.StatusNoContent, result.StatusCode)

		spans := mt.FinishedSpans()
		if !suite.Len(spans, 2) {
			return
		}

		{
			span := spans[0]
			suite.Equal("test-span", span.OperationName())
			suite.NotZero(span.ParentID())
		}

		{
			span := spans[1]
			suite.Equal("web.request", span.OperationName())
			suite.Equal("goyave.dev/goyave", span.Tag(ext.Component))
			suite.Equal("service-name", span.Tag(ext.ServiceName))
			suite.Equal("test", span.Tag(ext.Environment))
			suite.Equal(ext.SpanTypeWeb, span.Tag(ext.SpanType))
			suite.Equal(ext.SpanKindServer, span.Tag(ext.SpanKind))
			suite.Equal("/test", span.Tag(ext.HTTPURL))
			suite.Equal(http.MethodGet, span.Tag(ext.HTTPMethod))
			suite.Equal("test-route", span.Tag(ext.HTTPRoute))
		}
	})
}

func TestWriterSuite(t *testing.T) {
	err := config.LoadJSON(`{"app": {"environment": "test", "datadog": {"service": "service-name"}}}`)
	if !assert.NoError(t, err) {
		return
	}
	goyave.RunTest(t, new(MiddlewareTestSuite))
}

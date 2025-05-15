package goyavetrace

import (
	"encoding/json"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"goyave.dev/goyave/v5"
	"goyave.dev/goyave/v5/util/errors"
)

const componentName = "goyave.dev/goyave"

// Span tag names
const (
	TagUser = "user"
)

// Config common information for all traces.
type Config struct {
	// AgentAddr the address where the agent is located.
	AgentAddr string

	// Env name of the environment (tag `ext.Environment`)
	Env string

	// Service the name of the service (tag `ext.ServiceName`)
	Service string
}

// SpanOption function altering a span before finishing it. This can be used to add custom tags to the span.
type SpanOption func(*tracer.Span, *goyave.Response, *goyave.Request)

// DatadogUser minimal structure to identify a user at the origin of a trace.
type DatadogUser struct {
	Name  string `json:",omitempty"`
	Email string `json:",omitempty"`
	ID    int64
}

// String returns a JSON representation of the structure.
func (u DatadogUser) String() string {
	json, err := json.Marshal(u)
	if err != nil {
		panic(errors.New(err))
	}
	return string(json)
}

// DatadogUserConverter if be implemented by the user structure, a "user" tag will be added
// to the span with the returned `DatadogUser` structure as a value.
type DatadogUserConverter interface {
	ToDatadogUser() DatadogUser
}

// Start starts the tracer with the given set of options. The "agent address", "service" and "env" options
// are provided by default based on the following config entries:
//   - `app.datadog.agentAddr`
//   - `app.datadog.service`
//   - `app.environment`
//
// It will stop and replace any running tracer, meaning that calling it
// several times will result in a restart of the tracer by replacing the current instance with a new one.
func Start(cfg Config, opts ...tracer.StartOption) error {
	return tracer.Start(
		append([]tracer.StartOption{
			tracer.WithAgentAddr(cfg.AgentAddr),
			tracer.WithService(cfg.Service),
			tracer.WithEnv(cfg.Env),
		}, opts...)...,
	)
}

// Stop stops the started tracer. Subsequent calls are valid but become no-op.
func Stop() {
	tracer.Stop()
}

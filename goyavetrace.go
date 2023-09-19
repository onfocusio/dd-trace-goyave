package goyavetrace

import (
	"encoding/json"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"goyave.dev/goyave/v4/config"
)

// Span tag names
const (
	TagUser = "user"
)

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
		panic(err)
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
func Start(opts ...tracer.StartOption) {
	tracer.Start(
		append([]tracer.StartOption{
			tracer.WithAgentAddr(config.GetString("app.datadog.agentAddr")),
			tracer.WithService(config.GetString("app.datadog.service")),
			tracer.WithEnv(config.GetString("app.environment")),
		}, opts...)...,
	)
}

// Stop stops the started tracer. Subsequent calls are valid but become no-op.
func Stop() {
	tracer.Stop()
}

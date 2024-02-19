# dd-trace-goyave

Datadog trace client library for the Goyave framework (Onfocus fork). This provides utilities to start a background tracer and automatically trace all HTTP requests received.

The tracer connects to a locally running datadog agent. **This is required** for this library to work. By default, the tracer starts with the following options:
- Service name
- Environment

The middleware provided with this library traces all received HTTP requests with the following tags:
- Service name
- Environment
- Span type (web)
- Span kind (server)
- Request URL
- Request HTTP method
- Request route name
- Response status code
- Response error and stacktrace (if any)
- Response user (if any, and if supported)

## Usage

First, make sure the following configuration entries are set:
- `app.datadog.service`: used to set the service name in the span generated for each request.
- `app.datadog.agentAddr`: the datadog agent address to connect the tracer to. By default, the agent listens on `127.0.0.1:8126`.

In the `main` function, start the tracer after loading the config:
```go
import goyavetrace "github.com/onfocusio/dd-trace-goyave"

func main() {
    cfg := Config{
        AgentAddr: config.GetString("app.datadog.agentAddr"),
        Env:       config.GetString("app.environment"),
        Service:   config.GetString("app.datadog.service"),
    }
    goyavetrace.Start(cfg)
    defer goyavetrace.Stop()
}
```

The `Start()` function accepts a variadic slice of `tracer.StartOption` if you need to add more options.

**Be careful when using `os.Exit()` in the `main()`**: deferred calls are **not** executed if the program exits this way. You risk not flushing the tracer doing so.

Finally, in the main route registrer, add the global middleware so it is applied to all requests, even  if they don't match any route.
```go
import goyavetrace "github.com/onfocusio/dd-trace-goyave"

func Register(router *goyave.Router) {

    // Add custom tags to the spans
    spanOption := func(s tracer.Span, _ *goyave.Response, _ *goyave.Request) {
        s.SetTag(ext.ManualKeep, true)
    }

    cfg := Config{
        AgentAddr: config.GetString("app.datadog.agentAddr"),
        Env:       config.GetString("app.environment"),
        Service:   config.GetString("app.datadog.service"),
    }
    router.GlobalMiddleware(goyavetrace.NewMiddleware(cfg, spanOption))

    //...
}
```

### Tracing users

If the request has an authenticated user, their ID, name and email address can be added to the trace. To make it possible, the user structure must implement the `DatadogUserConverter` interface:

```go
type User struct {
	// ...
}

func (u User) ToDatadogUser() goyavetrace.DatadogUser {
	return goyavetrace.DatadogUser{
		ID:    u.ID,
		Name:  u.Name,
		Email: u.Email,
	}
}
```

If the user doesn't have a name or email (an API key for example), you can omit them and they won't be included in the tag.

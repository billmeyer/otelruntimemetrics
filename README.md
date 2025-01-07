# Golang Runtime Metric Collection using OpenTelemetry

In this example, we enable collection of [Go runtime metrics](https://pkg.go.dev/runtime/metrics) from an application using [OpenTelemetry](https://github.com/open-telemetry/opentelemetry-go-contrib/blob/main/instrumentation/runtime/example_test.go).

While the runtime metric collection works well, there are special considerations that must be made when collecting telemetry with an OpenTelemetry Gateway involved. Namely, the following Resource Attributes should be specified by the *application* (as opposed to deferring to the Collector to assign):

| Key                    |Value|
|------------------------|---|
| deployment.environment |dev|
| host.name              |myhostname|
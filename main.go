package main

import (
	"context"
	"fmt"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/sdk/metric"
	"log"
	"os"
	"os/signal"
	"time"

	"go.opentelemetry.io/otel"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const (
	serviceName    = "client"
	serviceVersion = "0.1.0"
	env            = "dev"
	otlpAddress    = "octodev03:4317"
	tracerName     = "otelruntimemetrics.service.tracer"
)

func newResource(ctx context.Context) (*sdkresource.Resource, error) {
	res, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
			semconv.DeploymentEnvironment(env),
			//semconv.DeploymentEnvironmentName(env), // When Semantic Convention v1.27.0 is supported at Datadog
		),
		sdkresource.WithSchemaURL(semconv.SchemaURL),
		sdkresource.WithFromEnv(), // pull attributes from OTEL_RESOURCE_ATTRIBUTES and OTEL_SERVICE_NAME environment variables
		sdkresource.WithHost(),    // This option configures a set of Detectors that discover host information
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	return sdkresource.Merge(sdkresource.Default(), res)
}

func main() {
	// See https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/runtime
	os.Setenv("OTEL_GO_X_DEPRECATED_RUNTIME_METRICS", "true")

	ctx := context.Background()
	res, err := newResource(ctx)

	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(otlpAddress),
	)
	if err != nil {
		log.Fatalf("failed to initialize exporter: %v", err)
	}

	// Create a new tracer provider with a batch span processor and the given exporter.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	// Handle shutdown properly so nothing leaks.
	defer func() { _ = tp.Shutdown(ctx) }()

	otel.SetTracerProvider(tp)

	// Metrics
	var metricProvider *metric.MeterProvider

	// Set to false to dump the metrics to stdout for troubleshooting
	const useOTLPMetricsExporter = true

	if useOTLPMetricsExporter == false {
		// Create an stdout Metric gRPC exporter for debugging
		metricExporter, err := stdoutmetric.New()
		if err != nil {
			log.Fatalf("failed to create metric exporter: %v", err)
		}
		read := metric.NewPeriodicReader(metricExporter, metric.WithInterval(10*time.Second))
		metricProvider = metric.NewMeterProvider(metric.WithResource(res), metric.WithReader(read))
	} else {
		// Create an OTLP Metric gRPC exporter
		metricExporter, err := otlpmetricgrpc.New(ctx,
			otlpmetricgrpc.WithInsecure(),
			otlpmetricgrpc.WithEndpoint(otlpAddress))
		if err != nil {
			log.Fatalf("failed to create metric exporter: %v", err)
		}

		// Register the exporter with an SDK via a periodic reader.
		read := metric.NewPeriodicReader(metricExporter, metric.WithInterval(10*time.Second))
		metricProvider = metric.NewMeterProvider(metric.WithResource(res), metric.WithReader(read))
	}

	otel.SetMeterProvider(metricProvider)
	defer func() {
		err := metricProvider.Shutdown(context.Background())
		if err != nil {
			log.Fatal(err)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	log.Print("Starting runtime instrumentation:")
	err = runtime.Start(runtime.WithMinimumReadMemStatsInterval(10 * time.Second))
	if err != nil {
		log.Fatal(err)
	}

	for true {
		fmt.Printf("Sending trace...\n")
		_, span := tp.Tracer(tracerName).Start(ctx, "MyTrace")
		span.End()

		fmt.Print("Sleeping...\n")
		time.Sleep(30 * time.Second)
	}
}

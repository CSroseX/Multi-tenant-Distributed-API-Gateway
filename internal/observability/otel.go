package observability

import (
    "context"
    "log"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func InitTracer(serviceName string) func() {
    exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
    if err != nil {
        log.Fatal(err)
    }

    // ✅ Use empty string for schemaURL as first arg
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            "", // schema URL (empty string for default)
            attribute.String("service.name", serviceName),
        )),
    )

    otel.SetTracerProvider(tp)

    return func() {
        _ = tp.Shutdown(context.Background())
    }
}

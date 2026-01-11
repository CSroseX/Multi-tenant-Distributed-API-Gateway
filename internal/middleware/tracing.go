package middleware

import (
    "net/http"

    "go.opentelemetry.io/otel"
)

func Tracing(next http.Handler) http.Handler {
    tracer := otel.Tracer("api-gateway")

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx, span := tracer.Start(r.Context(), r.URL.Path)
        defer span.End()

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

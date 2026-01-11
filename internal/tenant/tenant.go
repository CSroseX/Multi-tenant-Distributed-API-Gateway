package tenant

import (
    "context"
    "net/http"
)

// key type for context
type contextKey string

const tenantKey contextKey = "tenant"

// Tenant represents a simple tenant model
type Tenant struct {
    ID   string
    Name string
}

// Mock tenant DB (replace with real DB later)
var tenants = map[string]Tenant{
    "sk_test_123": {ID: "tenantA", Name: "Tenant A"},
    "sk_test_456": {ID: "tenantB", Name: "Tenant B"},
}

// FromContext returns tenant from request context
func FromContext(ctx context.Context) (*Tenant, bool) {
    t, ok := ctx.Value(tenantKey).(*Tenant)
    return t, ok
}

// Middleware extracts tenant from API key and stores in context
func Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        apiKey := r.Header.Get("X-API-Key")
        if apiKey == "" {
            http.Error(w, "Missing API Key", http.StatusUnauthorized)
            return
        }

        tenant, ok := tenants[apiKey]
        if !ok {
            http.Error(w, "Invalid API Key", http.StatusUnauthorized)
            return
        }

        // attach tenant to context
        ctx := context.WithValue(r.Context(), tenantKey, &tenant)
        r = r.WithContext(ctx)

        next.ServeHTTP(w, r)
    })
}

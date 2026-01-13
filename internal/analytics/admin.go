package analytics

import (
	"encoding/json"
	"net/http"
)

func Handler(a *Analytics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.URL.Query().Get("tenant")
		if tenantID == "" {
			http.Error(w, "tenant query missing", http.StatusBadRequest)
			return
		}

		data, _ := a.FetchTenantAnalytics(tenantID)
		json.NewEncoder(w).Encode(data)
	}
}

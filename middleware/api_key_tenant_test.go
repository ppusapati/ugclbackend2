package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"p9e.in/ugcl/config"
)

func TestAPIKeyTenantMiddleware(t *testing.T) {
	original := defaultTenantSchema
	t.Cleanup(func() { defaultTenantSchema = original })

	var gotSchema string
	handler := APIKeyTenantMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSchema = config.TenantSchemaFromContext(r.Context())
	}))

	tests := []struct {
		name          string
		defaultSchema string
		requestSchema string
		wantStatus    int
		wantSchema    string
	}{
		{"integration key keeps its own tenant", "sreeugcl", "othertenant", http.StatusOK, "othertenant"},
		{"static key falls back to default tenant", "sreeugcl", "", http.StatusOK, "sreeugcl"},
		{"static key rejected without default tenant", "", "", http.StatusUnauthorized, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultTenantSchema = tt.defaultSchema
			gotSchema = ""

			req := httptest.NewRequest(http.MethodGet, "/api/v1/partner/dprsite", nil)
			if tt.requestSchema != "" {
				req = req.WithContext(config.WithTenantSchema(req.Context(), tt.requestSchema))
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if gotSchema != tt.wantSchema {
				t.Errorf("handler saw tenant %q, want %q", gotSchema, tt.wantSchema)
			}
		})
	}
}

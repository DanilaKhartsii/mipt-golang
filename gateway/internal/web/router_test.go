package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gateway/internal/api"
)

func TestRouterServesInterfaceAndAPI(t *testing.T) {
	router := NewRouter(api.NewRouter(nil, time.Second))
	tests := []struct {
		path        string
		status      int
		contentType string
		body        string
	}{
		{path: "/", status: http.StatusOK, contentType: "text/html", body: "Пакетный импорт"},
		{path: "/assets/app.css", status: http.StatusOK, contentType: "text/css", body: ":root"},
		{path: "/assets/app.js", status: http.StatusOK, contentType: "javascript", body: "/api/transactions/bulk"},
		{path: "/ping", status: http.StatusOK, contentType: "application/json", body: `"pong"`},
		{path: "/api/transactions/bulk", status: http.StatusMethodNotAllowed, contentType: "application/json", body: "method not allowed"},
		{path: "/unknown", status: http.StatusNotFound, contentType: "application/json", body: "not found"},
		{path: "/assets/unknown.js", status: http.StatusNotFound, contentType: "application/json", body: "not found"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if rec.Code != tt.status || !strings.Contains(rec.Header().Get("Content-Type"), tt.contentType) || !strings.Contains(rec.Body.String(), tt.body) {
				t.Fatalf("response = %d, %q, %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
			}
		})
	}
}

package exporter_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jwkohnen/conntrack-stats-exporter/exporter"
)

func BenchmarkHandler(b *testing.B) {
	mockConntrackTool(b)

	var (
		r = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		w = new(nilResponseWriter)
	)

	for i := 0; i < b.N; i++ {
		exporter.Handler().ServeHTTP(w, r)
	}
}

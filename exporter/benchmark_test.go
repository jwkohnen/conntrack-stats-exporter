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
		w nullResponseWriter
	)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		exporter.Handler().ServeHTTP(w, r)
	}
}

func BenchmarkServeHTTP(b *testing.B) {
	mockConntrackTool(b)

	var (
		h = exporter.Handler()
		r = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		w nullResponseWriter
	)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h.ServeHTTP(w, r)
	}
}

type nullResponseWriter struct{}

func (nullResponseWriter) Write(p []byte) (int, error) { return len(p), nil }
func (nullResponseWriter) Header() http.Header         { return http.Header{} }
func (nullResponseWriter) WriteHeader(int)             {}

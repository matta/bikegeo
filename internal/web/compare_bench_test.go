package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// BenchmarkComparePage measures a full / request: query parsing, compare
// ranking, view building, and template rendering. `make profile` runs it with
// CPU and memory profiles; plain `go test ./...` skips benchmarks.
func BenchmarkComparePage(b *testing.B) {
	cases := []struct {
		name   string
		target string
	}{
		{"selected_bar", "/"},
		{"all_bars", "/?bar_options=all"},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			benchmarkPage(b, c.target)
		})
	}
}

// BenchmarkBikePage measures a full /bike request across default and all-options fitting.
func BenchmarkBikePage(b *testing.B) {
	cases := []struct {
		name   string
		target string
	}{
		{"default", "/bike"},
		{"all_forks_and_bars", "/bike?bar_options=all&fork_options=all"},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			benchmarkPage(b, c.target)
		})
	}
}

func benchmarkPage(b *testing.B, target string) {
	b.Helper()
	h := NewHandler()
	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.Host = "127.0.0.1:8080"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("GET %s: status %d", target, rec.Code)
		}
		b.SetBytes(int64(rec.Body.Len()))
	}
}

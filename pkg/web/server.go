// Package web serves bikegeo reports as stateless, JavaScript-free HTML pages.
//
// Every page is a pure function of the request URL: forms submit with GET,
// controls are populated from the parsed query, and nothing is stored between
// requests. Templates and CSS are embedded in the binary.
package web

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"
)

//go:embed templates/*.html static/*
var assets embed.FS

// NavItem is one entry in the site navigation.
type NavItem struct {
	Path, Label string
}

var nav = []NavItem{
	{"/", "Compare"},
	{"/frames", "Frames"},
	{"/forks", "Forks"},
	{"/stems", "Stems"},
	{"/ideals", "Ideals"},
	{"/saddles", "Saddles"},
	{"/saddle-swap", "Saddle swap"},
	{"/handlebars", "Handlebars"},
	{"/validate", "Validate"},
	{"/fit", "Fit"},
	{"/help", "Help"},
}

// base is embedded in every page's template data.
type base struct {
	Title  string
	Nav    []NavItem
	Active string
}

func newBase(title, active string) base {
	return base{Title: title, Nav: nav, Active: active}
}

type server struct {
	tpl *template.Template
}

// New returns the application handler. It serves only loopback Host headers.
func New() http.Handler {
	s := &server{
		tpl: template.Must(template.New("").Funcs(template.FuncMap{
			"lines": func(s string) []string { return strings.Split(s, "\n") },
		}).ParseFS(assets, "templates/*.html")),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.compare)
	mux.HandleFunc("GET /help", s.help)
	for _, p := range pages() {
		mux.HandleFunc("GET "+p.Path, s.page(p))
	}
	static, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	return hostGuard(mux)
}

// hostGuard rejects requests whose Host is not a loopback name, defeating DNS
// rebinding against the local server.
func hostGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		host = strings.Trim(host, "[]")
		switch strings.ToLower(host) {
		case "127.0.0.1", "localhost", "::1":
			next.ServeHTTP(w, r)
		default:
			http.Error(w, "bikegeo only serves loopback hosts", http.StatusForbidden)
		}
	})
}

// render executes the named template into a buffer first so a template error
// yields a clean 500 instead of a half-written page.
func (s *server) render(w http.ResponseWriter, status int, name string, data any) {
	var buf bytes.Buffer
	if err := s.tpl.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// Serve serves the application on ln until ctx is cancelled or serving fails.
func Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{
		Handler:           New(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		err := <-errCh
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

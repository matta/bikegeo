package web

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

// RunServe implements `bikegeo serve [--port N] [--open] [--render PATH [--include]]` and returns an exit code.
// The server binds to 127.0.0.1 only; --port 0 picks a free port.
// With --render, a single GET route is rendered in memory to out without opening a TCP socket.
func RunServe(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {}
	port := fs.Int("port", 8080, "TCP port to listen on (0 picks a free port)")
	open := fs.Bool("open", false, "Open the default browser once the server is listening")
	renderPath := fs.String("render", "", "Render one route path (e.g. / or /handlebars) in memory to stdout without binding a port")
	include := fs.Bool("include", false, "With --render, include HTTP status line and response headers before the body")
	printUsage := func(w io.Writer) {
		fmt.Fprintln(w, "Usage: bikegeo serve [--port N] [--open] | --render PATH [--include]")
		fs.SetOutput(w)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(out)
			return 0
		}
		printUsage(errOut)
		return 2
	}
	if fs.NArg() > 0 {
		if fs.NArg() == 1 && fs.Arg(0) == "help" {
			printUsage(out)
			return 0
		}
		fmt.Fprintf(errOut, "Error: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		printUsage(errOut)
		return 2
	}

	setFlags := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	if setFlags["render"] {
		if strings.TrimSpace(*renderPath) == "" {
			fmt.Fprintln(errOut, "Error: --render path cannot be empty")
			printUsage(errOut)
			return 2
		}
		if setFlags["open"] || setFlags["port"] {
			fmt.Fprintln(errOut, "Error: --render cannot be combined with --port or --open")
			printUsage(errOut)
			return 2
		}
		req, err := buildRenderRequest(*renderPath)
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return 2
		}
		if err := renderRoute(req, *include, out); err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return 1
		}
		return 0
	}

	if *include {
		fmt.Fprintln(errOut, "Error: --include requires --render")
		printUsage(errOut)
		return 2
	}
	if *port < 0 || *port > 65535 {
		fmt.Fprintf(errOut, "Error: invalid port %d\n", *port)
		return 2
	}

	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(*port)))
	if err != nil {
		fmt.Fprintf(errOut, "Error: %v\n", err)
		return 1
	}
	url := "http://" + ln.Addr().String() + "/"
	fmt.Fprintf(out, "bikegeo serving on %s (Ctrl-C to stop)\n", url)
	if *open {
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(errOut, "Warning: could not open browser: %v\n", err)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := Serve(ctx, ln); err != nil {
		fmt.Fprintf(errOut, "Error: %v\n", err)
		return 1
	}
	return 0
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

type memResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *memResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *memResponseWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
}

func (w *memResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(p)
}

func buildRenderRequest(rawPath string) (*http.Request, error) {
	if strings.Contains(rawPath, "://") {
		return nil, fmt.Errorf("--render expects a route path such as / or /handlebars, got %q", rawPath)
	}
	target := rawPath
	if !strings.HasPrefix(target, "/") {
		target = "/" + target
	}
	parsed, err := url.ParseRequestURI(target)
	if err != nil {
		return nil, fmt.Errorf("invalid route %q: %w", rawPath, err)
	}
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1", nil)
	if err != nil {
		return nil, err
	}
	req.URL.Path = parsed.Path
	req.URL.RawPath = parsed.RawPath
	req.URL.RawQuery = parsed.RawQuery
	req.RequestURI = parsed.RequestURI()
	return req, nil
}

func renderRoute(req *http.Request, includeHeaders bool, out io.Writer) error {
	rw := &memResponseWriter{}
	New().ServeHTTP(rw, req)
	status := rw.status
	if status == 0 {
		status = http.StatusOK
	}
	if includeHeaders {
		if _, err := fmt.Fprintf(out, "HTTP/1.1 %d %s\r\n", status, http.StatusText(status)); err != nil {
			return err
		}
		for _, k := range slices.Sorted(maps.Keys(rw.header)) {
			for _, v := range rw.header[k] {
				if _, err := fmt.Fprintf(out, "%s: %s\r\n", k, v); err != nil {
					return err
				}
			}
		}
		if _, err := io.WriteString(out, "\r\n"); err != nil {
			return err
		}
	}
	_, err := out.Write(rw.body.Bytes())
	return err
}

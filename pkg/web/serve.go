package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// RunServe implements `bikegeo serve [--port N] [--open]` and returns an exit code.
// The server binds to 127.0.0.1 only; --port 0 picks a free port.
func RunServe(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {}
	port := fs.Int("port", 8080, "TCP port to listen on (0 picks a free port)")
	open := fs.Bool("open", false, "Open the default browser once the server is listening")
	printUsage := func(w io.Writer) {
		fmt.Fprintln(w, "Usage: bikegeo serve [--port N] [--open]")
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

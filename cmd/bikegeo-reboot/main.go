package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"bikegeo/internal/reboot/web"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stdout, "Usage: bikegeo-reboot serve [--port N] [--open]")
		return 0
	}
	if args[0] != "serve" {
		fmt.Fprintf(os.Stderr, "Unknown command %q\nUsage: bikegeo-reboot serve [--port N] [--open]\n", args[0])
		return 2
	}

	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	port := fs.Int("port", 8081, "TCP port to listen on (0 picks a free port)")
	open := fs.Bool("open", false, "Open the default browser once the server is listening")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *port < 0 || *port > 65535 {
		fmt.Fprintf(os.Stderr, "Error: invalid port %d\n", *port)
		return 2
	}

	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(*port)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	url := "http://" + ln.Addr().String() + "/"
	fmt.Fprintf(os.Stdout, "bikegeo-reboot serving on %s (Ctrl-C to stop)\n", url)
	if *open {
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not open browser: %v\n", err)
		}
	}

	if err := http.Serve(ln, web.NewHandler()); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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

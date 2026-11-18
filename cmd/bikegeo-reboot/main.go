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

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
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

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleStemPreview)
	if err := http.Serve(ln, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func handleStemPreview(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	tr := pose.NewTrace()
	steererUp := pose.NewPose(pose.Point{X: 0, Y: 0}, pose.Deg(180.0-68.25), tr)
	placed, err := assembly.PlaceStem(steererUp, assembly.StemSetup{
		UpperCoverMm:  11.0,
		SpacersMm:     20.0,
		ClampHeightMm: 40.0,
		LengthMm:      60.0,
		AngleDeg:      -6.0,
	})
	tr.Freeze()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "bikegeo-reboot (Milestone 1 Slice 1a: Stem Pose & Trace)\n")
	fmt.Fprintf(w, "BarClamp: (%.2f, %.2f) mm @ %.2f° [step %d]\n\n",
		placed.BarClamp.Origin().X, placed.BarClamp.Origin().Y,
		placed.BarClamp.HeadingDeg(), placed.BarClamp.StepID())
	for _, s := range tr.Steps() {
		fmt.Fprintf(w, "#%d (parent %d) %-7s %-24s %-10s -> (%.2f, %.2f) @ %.2f°\n",
			s.ID, s.ParentID, s.Op, s.Label, s.Param, s.To.Origin.X, s.To.Origin.Y, s.To.HeadingDeg)
	}
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

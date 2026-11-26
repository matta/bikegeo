package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/matta/bikegeo/internal/compare"
	"github.com/matta/bikegeo/internal/query"
	"github.com/matta/bikegeo/internal/web"
)

const usage = `Usage:
  bikegeo serve [--port N] [--open]
  bikegeo compare [query]
  bikegeo bike [query]
`

const compareUsage = `Usage: bikegeo compare [query]

Ranks every size, fork, and sag of every catalog frame for the / compare
query and prints indented JSON. The query is URL-encoded; a leading "?" is
ignored. To refine one bike's size, fork, sag, roll, or pitch, use
bikegeo bike.

Query keys:
  top_per_frame, top       most rows kept per frame (default 1) / overall (0 keeps all)
  post, bar                seatpost and handlebar IDs
  bar_options              "selected" (default) or "all"
  brp_x, brp_y, brp        BRP target (mm); brp=off clears it
  grip_x, grip_y, grip     HandGrip target (mm); grip=off clears it

Examples:
  bikegeo compare
  bikegeo compare 'top_per_frame=0&top=10&post=setback_25'
`

const bikeUsage = `Usage: bikegeo bike [query]

Evaluates one catalog bike for the /bike query and prints indented JSON.
The query is URL-encoded; a leading "?" is ignored. With no query, the default
Otso Fenrir Stainless L build is evaluated against the default fit targets.

Query keys:
  frame, size                 catalog frame ID and size name
  fork, fork_a2c, cup, sag    fork ID ("basis" selects the chart basis fork), static A2C, lower cup stack, sag
  bar, spacers, stem_length, stem_angle, bar_roll
                              handlebar ID, spacer stack, stem length and angle, handlebar roll
  post, post_insert, rail_offset, rail_pitch
                              seatpost ID, insertion, saddle rail offset and pitch
  brp_x, brp_y, brp           BRP target (mm); brp=off clears it
  grip_x, grip_y, grip        HandGrip target (mm); grip=off clears it
  bar_options, fork_options   "selected" (default) or "all"
  view                        diagram view: bike, seat, or stem

Examples:
  bikegeo bike
  bikegeo bike 'frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20'
  bikegeo bike 'frame=binary-havok&size=L&brp=off&grip=off'
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run runs the CLI with compare ranking every catalog frame.
func run(args []string, stdout, stderr io.Writer) int {
	return runWith(query.CatalogFrames, args, stdout, stderr)
}

// runWith is run with compare ranking the frames compareFrames returns. Tests
// pass a few catalog frames trimmed to a size or fork.
func runWith(compareFrames func() []compare.Frame, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	switch args[0] {
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "compare":
		return runCompare(compareFrames, args[1:], stdout, stderr)
	case "bike":
		return runBike(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Unknown command %q\n%s", args[0], usage)
		return 2
	}
}

func runCompare(compareFrames func() []compare.Frame, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprint(stdout, compareUsage)
		return 0
	}
	if len(args) > 1 {
		fmt.Fprintln(stderr, "Usage: bikegeo compare [query]")
		return 2
	}
	raw := ""
	if len(args) == 1 {
		raw = strings.TrimPrefix(args[0], "?")
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		fmt.Fprintf(stderr, "Error: invalid query: %v\n", err)
		return 2
	}
	parsed, err := query.ParseCompare(values)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	parsed.Request.Frames = compareFrames()
	ranking, err := compare.Rank(parsed.Request)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	result, err := parsed.ResolveResult(ranking)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(stderr, "Error: encode JSON: %v\n", err)
		return 1
	}
	return 0
}

func runBike(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprint(stdout, bikeUsage)
		return 0
	}
	if len(args) > 1 {
		fmt.Fprintln(stderr, "Usage: bikegeo bike [query]")
		return 2
	}
	raw := ""
	if len(args) == 1 {
		raw = strings.TrimPrefix(args[0], "?")
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		fmt.Fprintf(stderr, "Error: invalid query: %v\n", err)
		return 2
	}
	parsed, err := query.ParseBike(values)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	result, err := parsed.Evaluate(query.RankedBuildLimit)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(stderr, "Error: encode JSON: %v\n", err)
		return 1
	}
	return 0
}

func runServe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	port := fs.Int("port", 8080, "TCP port to listen on (0 picks a free port)")
	open := fs.Bool("open", false, "Open the default browser once the server is listening")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *port < 0 || *port > 65535 {
		fmt.Fprintf(stderr, "Error: invalid port %d\n", *port)
		return 2
	}

	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(*port)))
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	url := "http://" + ln.Addr().String() + "/"
	fmt.Fprintf(stdout, "bikegeo serving on %s (Ctrl-C to stop)\n", url)
	if *open {
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(stderr, "Warning: could not open browser: %v\n", err)
		}
	}

	if err := http.Serve(ln, web.NewHandler()); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(stderr, "Error: %v\n", err)
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

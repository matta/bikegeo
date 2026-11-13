package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"bikegeo/pkg/web"
)

// App encapsulates the CLI runtime configuration and I/O streams.
type App struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// Run executes the bikegeo CLI with the provided arguments and I/O streams, returning an exit code.
func Run(args []string, in io.Reader, out, errOut io.Writer) int {
	app := &App{
		In:  in,
		Out: out,
		Err: errOut,
	}

	return app.dispatch(args)
}

// dispatch routes the remaining (global-flag-free) arguments to a subcommand.
func (app *App) dispatch(remaining []string) int {
	errOut := app.Err
	if len(remaining) == 0 {
		app.printUsage(app.Out)
		return 0
	}

	subcmd := strings.ToLower(remaining[0])
	subArgs := remaining[1:]

	switch subcmd {
	case "help":
		if len(subArgs) == 0 {
			app.printUsage(app.Out)
			return 0
		}
		target := strings.ToLower(subArgs[0])
		switch target {
		case "help", "-h", "--help":
			app.printUsage(app.Out)
			return 0
		case "list", "ls", "bikes", "frames":
			return app.runList([]string{"--help"})
		case "compare", "cmp":
			return app.runCompare([]string{"--help"})
		case "validate", "val", "check":
			return app.runValidate([]string{"--help"})
		case "fit":
			return app.runFit([]string{"--help"})
		case "serve":
			return web.RunServe([]string{"--help"}, app.Out, app.Err)
		default:
			fmt.Fprintf(errOut, "Error: unknown command %q\n\n", subArgs[0])
			app.printUsage(errOut)
			return 2
		}
	case "-h", "--help":
		app.printUsage(app.Out)
		return 0
	case "list", "ls", "bikes", "frames":
		return app.runList(subArgs)
	case "compare", "cmp":
		return app.runCompare(subArgs)
	case "validate", "val", "check":
		return app.runValidate(subArgs)
	case "fit":
		return app.runFit(subArgs)
	case "serve":
		return web.RunServe(subArgs, app.Out, app.Err)
	default:
		fmt.Fprintf(errOut, "Error: unknown command %q\n\n", subcmd)
		app.printUsage(errOut)
		return 2
	}
}

func (a *App) printUsage(w io.Writer) {
	fmt.Fprint(w, `BikeGeo Analyst CLI - Bicycle Geometry Validation & Apples-to-Apples Comparison

Usage:
  bikegeo <command> [flags] [arguments]

Core Commands:
  compare [spec...]       Compare bike setups (frame[@size][@fork][@sag%]) or run default comparison
  validate [frame...]     Verify CAD Master Stack Equation, publish mode, and multi-config coherence
  fit <frame[:size]>      Analyze CAD residuals, solve minimax A2C/spacer, and sweep parameter trials
  list                    List all bike frames, sizes, and supported fork capacities
  serve [--port N] [--open] | --render PATH [--include]
                          Serve the human-facing web UI on 127.0.0.1 (or render one route to stdout)

Examples:
  # Run the default comparison matrix:
  bikegeo compare

  # Compare all registered frames across all plausible configurations (top 1 per frame):
  bikegeo compare --all

  # Compare the same frame at 20% sag, 30% sag, and with its rigid fork:
  bikegeo compare hayduke@20% hayduke@30% hayduke@rigid

  # Compare different frames at different sag points and fork capacities:
  bikegeo compare revel-el-jefe@120mm@20% hayduke@130mm@30% fenrir@rigid

  # Compare all sizes of a frame against each other with a specified fork:
  bikegeo compare seven-expedition-926f:all@120mm

  # Add all sizes of a frame to the default comparison against your reference bike:
  bikegeo compare +seven-expedition-926f:all@120mm

  # Validate a frame's CAD math and cross-configuration coherence:
  bikegeo validate otso-fenrir-stainless
  bikegeo validate --all

  # Launch the web UI (catalogs, saddle swap, handlebar and comparison tools):
  bikegeo serve --open
`)
}

func (a *App) parseSubcommandFlags(
	fs *flag.FlagSet,
	args []string,
	boolFlags map[string]bool,
	allowRemovalTokens bool,
	helpUsage, errUsage func(io.Writer),
) (bool, int) {
	fs.SetOutput(a.Err)
	fs.Usage = func() {}
	reordered := reorderFlagsForStdlib(fs, args, boolFlags, allowRemovalTokens)
	if err := fs.Parse(reordered); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			helpUsage(a.Out)
			return false, 0
		}
		errUsage(a.Err)
		return false, 2
	}
	return true, 0
}

// reorderFlagsForStdlib moves flag arguments ahead of positional arguments so stdlib flag.FlagSet
// parses flags regardless of whether they appear before or after positional arguments.
// When allowRemovalTokens is true (compare), single-dash tokens other than "-h" (e.g., "-2",
// "-sendero", "-all") are treated as positional removal specs rather than flags.
func reorderFlagsForStdlib(fs *flag.FlagSet, args []string, boolFlags map[string]bool, allowRemovalTokens bool) []string {
	var flags []string
	var pos []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			pos = append(pos, arg)
			continue
		}

		isDoubleDash := strings.HasPrefix(arg, "--")
		rawName := strings.TrimPrefix(arg, "-")
		if isDoubleDash {
			rawName = strings.TrimPrefix(arg, "--")
		}
		name, _, hasEq := strings.Cut(rawName, "=")

		if allowRemovalTokens && !isDoubleDash && name != "h" {
			// Single-dash token in compare (e.g., "-2", "-sendero", "-all") is a column removal spec.
			pos = append(pos, arg)
			continue
		}

		flags = append(flags, arg)
		if fs.Lookup(name) != nil && !hasEq && !boolFlags[name] && name != "h" && name != "help" && i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}

	if len(pos) > 0 {
		return append(append(flags, "--"), pos...)
	}
	return flags
}

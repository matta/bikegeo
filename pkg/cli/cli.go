package cli

import (
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
		app.printUsage()
		return 0
	}

	subcmd := strings.ToLower(remaining[0])
	subArgs := remaining[1:]

	switch subcmd {
	case "help", "-h", "--help":
		app.printUsage()
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
		app.printUsage()
		return 2
	}
}

func (a *App) printUsage() {
	fmt.Fprint(a.Out, `BikeGeo Analyst CLI - Bicycle Geometry Validation & Apples-to-Apples Comparison

Usage:
  bikegeo <command> [flags] [arguments]

Core Commands:
  compare [spec...]       Compare bike setups (frame[@size][@fork][@sag%]) or run default comparison
  validate [frame...]     Verify CAD Master Stack Equation, publish mode, and multi-config coherence
  fit <frame[:size]>      Analyze CAD residuals, solve minimax A2C/spacer, and sweep parameter trials
  list                    List all bike frames, sizes, and supported fork capacities
  serve [--port N] [--open]
                          Serve the human-facing web UI on 127.0.0.1 (default port 8080)

Examples:
  # Run the default comparison matrix:
  bikegeo compare

  # Compare all registered frames across all plausible configurations (top 1 per frame):
  bikegeo compare --all

  # Compare the same frame at 20% sag, 30% sag, and with its rigid fork:
  bikegeo compare sendero@20% sendero@30% sendero@rigid

  # Compare different frames at different sag points and fork capacities:
  bikegeo compare sendero@120mm@20% chameleon@130mm@30% fenrir@rigid

  # Compare all sizes of a frame against each other with a specified fork:
  bikegeo compare seven-expedition-926f:all@120mm

  # Add all sizes of a frame to the default comparison against your reference bike:
  bikegeo compare +seven-expedition-926f:all@120mm

  # Validate a frame's CAD math and cross-configuration coherence:
  bikegeo validate chumba-sendero
  bikegeo validate --all

  # Analyze swapping from reference SQlab 610 to Berthoud Aspin:
  bikegeo saddle-swap
  bikegeo saddle-swap sqlab slr
`)
}

// reorderFlagsForStdlib moves flag arguments ahead of positional arguments so stdlib flag.FlagSet
// parses flags regardless of whether they appear before or after positional arguments.
// Notice that negative numbers or "-<col_num>" incremental remove tokens are treated as positional args if not registered as flags.
func reorderFlagsForStdlib(fs *flag.FlagSet, args []string, boolFlags map[string]bool) []string {
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

		name := strings.TrimLeft(arg, "-")
		hasEq := strings.Contains(name, "=")
		if hasEq {
			name = strings.SplitN(name, "=", 2)[0]
		}

		if fs.Lookup(name) == nil && name != "h" && name != "help" {
			// Not a known flag (e.g., "-2" or "-sendero" column removal token)
			pos = append(pos, arg)
			continue
		}

		flags = append(flags, arg)
		if !hasEq && !boolFlags[name] && name != "h" && name != "help" && i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}

	if len(pos) > 0 {
		return append(append(flags, "--"), pos...)
	}
	return flags
}

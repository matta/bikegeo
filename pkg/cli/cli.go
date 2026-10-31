package cli

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"bikegeo/pkg/model"
	"bikegeo/pkg/storage"
)

// App encapsulates the CLI runtime configuration and I/O streams.
type App struct {
	DataDir   string
	StatePath string
	Format    string
	In        io.Reader
	Out       io.Writer
	Err       io.Writer
}

// Run executes the bikegeo CLI with the provided arguments and I/O streams, returning an exit code.
func Run(args []string, in io.Reader, out, errOut io.Writer) int {
	app := &App{
		DataDir: "data/bikes",
		Format:  FormatTable,
		In:      in,
		Out:     out,
		Err:     errOut,
	}

	remaining, err := app.extractGlobalFlags(args)
	if err != nil {
		fmt.Fprintf(errOut, "Error: %v\n", err)
		return 2
	}

	if app.StatePath == "" {
		parent := filepath.Dir(filepath.Clean(app.DataDir))
		app.StatePath = filepath.Join(parent, "last_compare.json")
	}

	if _, err := NormalizeFormat(app.Format); err != nil {
		fmt.Fprintf(errOut, "Error: %v\n", err)
		return 2
	}

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
	case "forks":
		return app.runForks(subArgs)
	case "stems":
		return app.runStems(subArgs)
	case "ideals", "ideal":
		return app.runIdeals(subArgs)
	case "compare", "cmp":
		return app.runCompare(subArgs)
	case "validate", "val", "check":
		return app.runValidate(subArgs)
	case "fit":
		return app.runFit(subArgs)
	case "init", "template", "new":
		return app.runInit(subArgs)
	case "set":
		return app.runSet(subArgs)
	case "import":
		return app.runImport(subArgs)
	case "delete", "rm", "del":
		return app.runDelete(subArgs)
	default:
		fmt.Fprintf(errOut, "Error: unknown command %q\n\n", subcmd)
		app.printUsage()
		return 2
	}
}

func (a *App) extractGlobalFlags(args []string) ([]string, error) {
	var remaining []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--data-dir":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--data-dir requires a path argument")
			}
			a.DataDir = args[i+1]
			i++
		case strings.HasPrefix(arg, "--data-dir="):
			a.DataDir = strings.TrimPrefix(arg, "--data-dir=")
		case arg == "--state-file":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--state-file requires a path argument")
			}
			a.StatePath = args[i+1]
			i++
		case strings.HasPrefix(arg, "--state-file="):
			a.StatePath = strings.TrimPrefix(arg, "--state-file=")
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("%s requires a format argument (table, json, md, csv)", arg)
			}
			a.Format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			a.Format = strings.TrimPrefix(arg, "--format=")
		case arg == "--json":
			a.Format = FormatJSON
		default:
			remaining = append(remaining, arg)
		}
	}
	return remaining, nil
}

func (a *App) openStore() (*storage.BikeStore, error) {
	return storage.NewBikeStore(a.DataDir)
}

func (a *App) printUsage() {
	fmt.Fprint(a.Out, `BikeGeo Analyst CLI - Bicycle Geometry Validation & Apples-to-Apples Comparison

Usage:
  bikegeo <command> [flags] [arguments]

Core Commands:
  compare [spec...]       Compare bike setups (frame[@size][@fork][@sag%]) or replay last comparison
  validate [frame...]     Verify CAD Master Stack Equation, publish mode, and multi-config coherence
  fit <frame[:size]>      Analyze CAD residuals, solve minimax A2C/spacer, and sweep parameter trials
  list                    List all stored bike frames, sizes, and supported fork capacities
  forks                   List standard 29er fork presets and travel-to-A2C mappings
  stems                   List standard stem presets and length/angle envelopes
  ideals [show|edit [name]] List, inspect, or edit ideal frame profiles in a TUI

Data Management Commands:
  init                    Generate a starter FrameModel JSON template
  set <frame[:size]> ...  Update frame or geometry fields (e.g. reach=475 stack=645)
  import <file.json>      Validate and import a frame JSON file into data/bikes/
  delete <frame[:size]>   Delete a frame or remove a single size from a frame

Global Flags:
  --data-dir <dir>        Path to bike JSON directory (default: "data/bikes")
  --state-file <file>     Path to saved comparison state (default: "data/last_compare.json")
  -f, --format <fmt>      Output format: table (default), json, md, csv
  --json                  Shorthand for --format json

Examples:
  # Replay the most recent comparison stored in data/last_compare.json:
  bikegeo compare

  # Compare the same frame at 20% sag, 30% sag, and with its rigid fork:
  bikegeo compare sendero@20% sendero@30% sendero@rigid

  # Compare different frames at different sag points and fork capacities:
  bikegeo compare sendero@120mm@20% chameleon@130mm@30% fenrir@rigid

  # Compare all sizes of a frame against each other with a specified fork:
  bikegeo compare seven-expedition-926f:all@120mm

  # Add all sizes of a frame to the current comparison against your reference bike:
  bikegeo compare +seven-expedition-926f:all@120mm

  # Launch interactive prompt/response builder to edit or run comparison:
  bikegeo compare -i

  # Validate a frame's CAD math and cross-configuration coherence:
  bikegeo validate chumba-sendero
  bikegeo validate --all
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

// ResolveFrameByToken finds a FrameModel by exact ID, legacy size-suffixed ID, or unique substring match.
// If the token included a size suffix (e.g. "chumba-sendero-l" or "chumba-sendero:L"), inferredSize is returned.
func ResolveFrameByToken(frames []model.FrameModel, token string) (*model.FrameModel, string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, "", fmt.Errorf("empty frame identifier")
	}

	var explicitSize string
	if strings.Contains(token, ":") {
		parts := strings.SplitN(token, ":", 2)
		token = parts[0]
		explicitSize = parts[1]
	}

	lower := strings.ToLower(token)

	// 1. Exact frame ID match
	for i := range frames {
		if strings.ToLower(frames[i].ID) == lower {
			return &frames[i], explicitSize, nil
		}
	}

	// 2. Exact match on frame ID + "-<size>"
	for i := range frames {
		for _, sz := range frames[i].GetSizes() {
			cand := strings.ToLower(fmt.Sprintf("%s-%s", frames[i].ID, sz))
			if cand == lower {
				if explicitSize == "" {
					explicitSize = sz
				}
				return &frames[i], explicitSize, nil
			}
		}
	}

	// 3. Exact match on Model or Manufacturer + Model
	for i := range frames {
		full := strings.ToLower(fmt.Sprintf("%s %s", frames[i].Manufacturer, frames[i].Model))
		if strings.ToLower(frames[i].Model) == lower || full == lower {
			return &frames[i], explicitSize, nil
		}
	}

	// 4. Substring match across ID, Manufacturer, Model
	var matches []*model.FrameModel
	for i := range frames {
		haystack := strings.ToLower(fmt.Sprintf("%s %s %s", frames[i].ID, frames[i].Manufacturer, frames[i].Model))
		if strings.Contains(haystack, lower) {
			matches = append(matches, &frames[i])
		}
	}

	if len(matches) == 1 {
		return matches[0], explicitSize, nil
	}
	if len(matches) > 1 {
		var ids []string
		for _, m := range matches {
			ids = append(ids, m.ID)
		}
		return nil, "", fmt.Errorf("ambiguous frame %q matches multiple frames: %s", token, strings.Join(ids, ", "))
	}

	var available []string
	for _, f := range frames {
		available = append(available, f.ID)
	}
	return nil, "", fmt.Errorf("frame %q not found (available: %s)", token, strings.Join(available, ", "))
}

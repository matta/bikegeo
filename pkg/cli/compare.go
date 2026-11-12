package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"bikegeo/pkg/compare"
)

func (a *App) runCompare(args []string) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	fs.Usage = func() {
		a.printCompareUsage(a.Out)
	}

	var transposeFlag bool
	var noScoreFlag bool
	var scoreDetailFlag bool
	var saddleFlag string
	var handlebarFlag string
	var allFlag bool
	var allConfigsFlag bool
	var topPerFrameFlag int
	var topLimitFlag int

	fs.BoolVar(&transposeFlag, "transpose", false, "Transpose table (Columns as rows, Metrics as columns)")
	fs.BoolVar(&noScoreFlag, "no-score", false, "Disable scoring and ranking display")
	fs.BoolVar(&scoreDetailFlag, "score-detail", false, "Display detailed scoring breakdown table")
	fs.StringVar(&saddleFlag, "saddle", "", "Saddle model from catalog (default: 'sqlab-610-ergolux')")
	fs.StringVar(&handlebarFlag, "handlebar", "", "Handlebar model from catalog (default: 'persuader20_800')")
	fs.BoolVar(&allFlag, "all", false, "Compare all registered frames across all sizes and plausible configurations")
	fs.BoolVar(&allConfigsFlag, "all-configs", false, "Expand all sizes and plausible fork/sag configurations for target frame(s)")
	fs.IntVar(&topPerFrameFlag, "top-per-frame", -1, "Maximum manifestations kept per frame (default: 1 for --all, 5 for specific frame @all, 0 for unlimited)")
	fs.IntVar(&topLimitFlag, "top", 0, "Maximum total comparison setups to display after ranking (0 for unlimited)")

	boolFlags := map[string]bool{
		"transpose":    true,
		"no-score":     true,
		"score-detail": true,
		"all":          true,
		"all-configs":  true,
	}
	reordered := reorderFlagsForStdlib(fs, args, boolFlags)
	if err := fs.Parse(reordered); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	explicitFlags := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

	posArgs := fs.Args()
	if len(posArgs) == 1 && (posArgs[0] == "help" || posArgs[0] == "-h" || posArgs[0] == "--help") {
		a.printCompareUsage(a.Out)
		return 0
	}

	opts := compare.Options{
		Specs:      posArgs,
		Saddle:     saddleFlag,
		Handlebar:  handlebarFlag,
		All:        allFlag,
		AllConfigs: allConfigsFlag,
		Top:        topLimitFlag,
	}
	if explicitFlags["transpose"] {
		opts.Transpose = &transposeFlag
	}
	if explicitFlags["top-per-frame"] {
		opts.TopPerFrame = &topPerFrameFlag
	}

	res, err := compare.Run(opts)
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}

	if err := a.renderComparison(res, !noScoreFlag, scoreDetailFlag); err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	return 0
}

// renderComparison lays out res as a report document and prints it in a.Format.
func (a *App) renderComparison(res *compare.Result, scoringEnabled, scoreDetail bool) error {
	// Flat keeps a many-bike comparison as one wide table, which is the
	// easiest layout for a program to parse.
	doc, err := compare.BuildDocument(res, compare.DocOptions{
		Scoring: scoringEnabled,
		Detail:  scoreDetail,
		Flat:    true,
	})
	if err != nil {
		return err
	}
	a.printDocument(doc)
	return nil
}

func (a *App) printCompareUsage(w io.Writer) {
	fmt.Fprint(w, `BikeGeo Analyst CLI - Compare Geometry & Fit Scoring

Usage:
  bikegeo compare [flags] [spec...]
  bikegeo compare --all [flags]
  bikegeo compare [flags] +<spec> ...
  bikegeo compare [flags] -<spec|col_num> ...

Overview:
  The compare command performs an apples-to-apples kinematic comparison of
  bicycle frame geometries, normalizing dynamic fork travel, axle-to-crown (A2C),
  rake, headset cup spacers, and suspension sag. Each setup is evaluated against
  the rider's biomechanical profile (saddle height, back angle, arm reach, and
  center of mass), computing key fit metrics (effective reach/stack, actual setback,
  AETT, seat-axis-to-head-tube drop, front weight distribution) and scoring each setup
  against ideal fit criteria.

Comparison Specs:
  Comparison columns are defined using flexible spec tokens:
    [+|-]frame[:size][@mod1][@mod2]...
    (or using '@' throughout: frame[@size][@fork][@sag])

  Frame Identifier:
    - Exact frame ID (e.g. "seven-expedition-926f", "binary-havok")
    - Model name or unique substring match (e.g. "expedition", "fenrir", "krampus")
    - All frames ("all"): Expands comparison across all registered frames in memory.
      Frames that do not support a requested size or fork modifier are gracefully skipped.

  Size Specifier:
    - Specific size (e.g. ":L", ":XL", ":S3", ":S 60cm"): Must match one of the
      frame's declared sizes.
    - Default size (omitted size): If no size is specified, the frame's declared
      default_size is automatically selected.
    - All sizes (":all" or "@all"): Expands the frame into individual comparison
      columns for EVERY size declared by the manufacturer.

  Modifiers (order-independent, separated by '@'):
    - Fork travel (e.g. "@120mm", "@100mm", or "@120"):
      Suspension travel in millimeters. Must be within the frame's supported forks.
    - Rigid fork ("@rigid" or "@0mm"):
      Configures the frame with its declared rigid fork.
      Only permitted if the frame supports rigid forks. Rigid forks require 0% sag.
    - Fork preset ID (e.g. "@enve_472", "@stock"):
      Selects a known preset fork from the catalog.
    - All plausible forks ("@all", "@all-forks", or "@forks"):
      Expands the frame across all supported rigid forks and all 10mm suspension travel
      steps in its supported range, across plausible sags (20%, 25%, 30%).
    - Sag percentage (e.g. "@20%", "@sag=25", or "@20"):
      Suspension sag percent. Pure numbers < 60 are treated as sag %; numbers >= 60
      are treated as fork travel in mm. If omitted, uses the frame's default sag.
    - Default fork (omitted fork):
      If no fork is specified, the frame's default fork (first entry in forks) is used.

Comparison State & Column Management:
  Default Comparison Baseline:
    - Running 'bikegeo compare' with no arguments evaluates the compiled default comparison setups.

  All-Frames & All-Configurations Expansion:
    - 'bikegeo compare --all' (or 'bikegeo compare all'):
      Compares all registered frames across all sizes and plausible fork/sag configurations,
      automatically selecting the #1 highest-scoring manifestation per frame (18 rows).
    - 'bikegeo compare <frame>:all@all' (or 'bikegeo compare --all-configs <frame>'):
      Compares all sizes and plausible fork/sag configurations for that frame (default: top 5).
    - 'bikegeo compare all@rigid':
      Compares all registered frames supporting rigid forks (default size, rigid fork).

  Two-Stage Ranking & Pruning Funnel:
    - Stage 1 (Optimal Sag): When expanding plausible forks and sags, each (Frame, Size, Fork)
      selects its single highest-scoring sag point (breaking score ties toward 20% sag).
    - Stage 2 (Top N per Frame): Keeps at most N manifestations per frame (--top-per-frame).
      Defaults to 1 for all-frames comparison and 5 for specific frame @all sweeps.
    - Stage 3 (Global Top Limit): Optionally restricts output to top M setups overall (--top).

  Full Replacement (Bare Specs):
    - Specifying arguments without '+' or '-' replaces all columns for that run.
      Example: bikegeo compare fenrir:L@110mm hayduke:S3@120mm

  Incremental Addition ('+spec'):
    - Prefixing a spec with '+' appends column(s) to the default comparison baseline.
      Example: bikegeo compare +seven-expedition-926f:all@120mm

  Incremental Removal ('-spec' or '-col_num'):
    - By column number: '-1', '-2', etc. removes the 1-based column from the default baseline.
    - By frame ID / substring: '-fenrir' or '-fenrir:all' removes all columns of that frame.
    - By frame size: '-fenrir:L' removes all columns of that frame in size L.
    - By specific fork/sag: '-fenrir:L@110mm' removes only that matching setup.
    - All columns: '-all' removes all columns.

Table Layout & Auto-Transposition:
  - Standard View (Columns): Each bike is a vertical column; metrics are rows.
    Ideal and Acceptable ranges appear as reference columns.
  - Transposed View (Rows): Each bike is a row; metrics are columns.
    Ideal and Acceptable ranges appear as top header rows.
    Enabled with --transpose.
  - Automatic Transposition: When a comparison has more than 4 columns (such as
    expanding ':all' or '--all'), the table automatically renders in transposed mode unless
    explicitly disabled with --transpose=false.

Scoring & Ranking:
  - Setups are scored against sweet-spot curves defined in an Ideal Profile (default: 'default').
  - Metrics display their evaluated zone (✓ ideal, ~ acceptable, ✗ outside) and distance to ideal.
  - Displayed rows are sorted descending by total score, placing the top-match #1 at the top.
  - Explanations section highlights Biggest Losses vs. Ideal and Head-to-Head Deciding Factors.
  - Use --score-detail to view the full criteria breakdown table.
  - Use --no-score to disable scoring and view columns in their original configuration order.

Flags:
  --all                        Compare all registered frames across all plausible configurations
  --all-configs                Expand all sizes and plausible fork/sag configurations for target frame(s)
  --top-per-frame <N>          Maximum manifestations kept per frame (default: 1 for --all, 5 for @all, 0=all)
  --top <M>                    Maximum total comparison setups to display after ranking (0 for unlimited)
  --saddle <name>              Select saddle model from catalog ('sqlab-610-ergolux' or 'berthoud-aspin-open')
  --handlebar <name>           Select handlebar model from catalog (default: 'persuader20_800')
  --transpose                  Render transposed table (Columns as rows, Metrics as columns)
  --score-detail               Display detailed scoring breakdown table for each criterion
  --no-score                   Disable scoring and ranking display
`)
}

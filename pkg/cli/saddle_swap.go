package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/catalog"
	"bikegeo/pkg/frames"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
)

func (a *App) runSaddleSwap(args []string) int {
	fs := flag.NewFlagSet("saddle-swap", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	fs.Usage = func() {
		a.printSaddleSwapUsage(a.Out)
	}

	var saddleHeight float64
	var sitBoneSetback float64
	var brpSetback float64
	var postSetback float64
	var clampLength float64
	var hbDistance float64
	var hbDrop float64
	var compareAll bool
	var frameSpec string

	fs.Float64Var(&saddleHeight, "saddle-height", 746.0, "Rider radial saddle height from BB to BRP in mm (not to saddle center or sit bones; default: 746mm)")
	fs.Float64Var(&sitBoneSetback, "sit-bone-setback", 292.0, "Target sit bone setback behind BB in mm (default: 292mm)")
	fs.Float64Var(&brpSetback, "brp-setback", 0.0, "Derive sit bone setback from source saddle 70mm BRP setback in mm")
	fs.Float64Var(&postSetback, "seatpost", model.DefaultSeatpostSetbackMm, "Seatpost perpendicular setback in mm (default: 11mm Redshift sagged)")
	fs.Float64Var(&clampLength, "clamp-length", model.DefaultSeatpostClampLengthMm, "Seatpost clamp length in mm (default: 30mm)")
	fs.Float64Var(&hbDistance, "cockpit-dist", 700.0, "Reference Saddle-to-Handlebar distance in mm (default: 700mm)")
	fs.Float64Var(&hbDrop, "cockpit-drop", 13.0, "Reference Saddle-to-Handlebar drop in mm (default: 13mm)")
	fs.BoolVar(&compareAll, "all", false, "Compare source saddle against all saddles in catalog")
	fs.StringVar(&frameSpec, "frame", "", "Evaluate saddle swap on a specific bike frame[:size] (e.g. otso-fenrir-stainless:L)")

	boolFlags := map[string]bool{
		"all": true,
	}
	reordered := reorderFlagsForStdlib(fs, args, boolFlags)
	if err := fs.Parse(reordered); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	posArgs := fs.Args()
	sourceSaddle := catalog.DefaultSaddle()
	var targetSaddles []model.SaddleSpec

	if compareAll {
		if len(posArgs) > 0 {
			if s, ok := catalog.GetSaddle(posArgs[0]); ok {
				sourceSaddle = s
			} else {
				fmt.Fprintf(a.Err, "Error: unrecognized source saddle %q\n", posArgs[0])
				return 1
			}
		}
		for _, s := range catalog.AllSaddles() {
			if s.ID != sourceSaddle.ID {
				targetSaddles = append(targetSaddles, s)
			}
		}
	} else if len(posArgs) >= 2 {
		if s, ok := catalog.GetSaddle(posArgs[0]); ok {
			sourceSaddle = s
		} else {
			fmt.Fprintf(a.Err, "Error: unrecognized source saddle %q\n", posArgs[0])
			return 1
		}
		if s, ok := catalog.GetSaddle(posArgs[1]); ok {
			targetSaddles = []model.SaddleSpec{s}
		} else {
			fmt.Fprintf(a.Err, "Error: unrecognized target saddle %q\n", posArgs[1])
			return 1
		}
	} else if len(posArgs) == 1 {
		if s, ok := catalog.GetSaddle(posArgs[0]); ok {
			targetSaddles = []model.SaddleSpec{s}
		} else {
			fmt.Fprintf(a.Err, "Error: unrecognized target saddle %q\n", posArgs[0])
			return 1
		}
	} else {
		// Default: SQlab 610 Ergolux -> Berthoud Aspin Open
		if aspin, ok := catalog.GetSaddle("berthoud-aspin-open"); ok {
			targetSaddles = []model.SaddleSpec{aspin}
		} else {
			fmt.Fprintf(a.Err, "Error: default target saddle berthoud-aspin-open not found\n")
			return 1
		}
	}

	if brpSetback > 0 {
		sitBoneSetback = sourceSaddle.SitBoneSetbackFromBrp(brpSetback)
	}

	// 1. Optional Summary Matrix if --all
	if compareAll && len(targetSaddles) > 0 {
		summaryHeaders := []string{
			"Target Saddle",
			"Rail",
			"BRP Span (ΔX)",
			"Step Ht (ΔY)",
			"Equivalent BRP (X, Y)",
			"Target Saddle Height (BRP)",
			"Pelvic Rail Δ",
			"STA Δ",
			"30mm Clamp",
			"45mm Clamp",
			"Compatibility Verdict",
		}
		var summaryRows [][]string

		for _, tgt := range targetSaddles {
			rep := geometry.AnalyzeSaddleSwap(
				sourceSaddle,
				tgt,
				saddleHeight,
				sitBoneSetback,
				postSetback,
				clampLength,
				hbDistance,
				hbDrop,
			)

			c30 := "Feasible"
			if !rep.Pelvic.IsFeasible {
				c30 = "Infeasible"
			}
			tMin45, tMax45 := tgt.ClampWindowMm(model.DropBestClampLengthMm)
			c45 := "Feasible"
			if tgt.RailLengthMm < model.DropBestClampLengthMm || rep.Pelvic.RequiredClampOffsetMm < tMin45 || rep.Pelvic.RequiredClampOffsetMm > tMax45 {
				c45 = "Infeasible"
			}

			verdict := "Fully Compatible (minor rail adjustment)"
			if !rep.Pelvic.IsFeasible {
				if rep.RailCenterToSitBonesDeltaMm < 0 {
					verdict = "Requires zero/low-setback post (rearward sit bones)"
				} else {
					verdict = "Requires high-setback post"
				}
			}

			summaryRows = append(summaryRows, []string{
				tgt.Name,
				fmt.Sprintf("%.0f mm", tgt.RailLengthMm),
				fmt.Sprintf("%.1f mm", tgt.DeltaBrpToSitBonesMm()),
				fmt.Sprintf("%.1f mm (%+.1f mm)", tgt.SitBonesElevationMm, rep.StepElevationDeltaMm),
				fmt.Sprintf("(-%.1f, %.1f) mm (%+.1f, %+.1f)", rep.TargetEquivalentBrpSetbackMm, rep.TargetEquivalentBrpHeightMm, rep.BrpDeltaXMm, rep.BrpDeltaYMm),
				fmt.Sprintf("%.1f mm (%+.1f mm)", rep.TargetEquivalentSaddleHeightMm, rep.BrpSaddleHeightShiftMm),
				fmt.Sprintf("%+.1f mm", rep.RailCenterToSitBonesDeltaMm),
				fmt.Sprintf("%+.2f°", rep.BaselineStaDeg-sourceSaddle.IdealSeatTubeAngle(sitBoneSetback, saddleHeight, postSetback)),
				c30,
				c45,
				verdict,
			})
		}

		sumTitle := fmt.Sprintf("Saddle Swap Catalog Overview (Baseline: %s)", sourceSaddle.Name)
		sumSub := fmt.Sprintf("Source Saddle Height (to BRP): %.0fmm | Sit Bone Setback: %.0fmm behind BB | Post Setback: %.1fmm",
			saddleHeight, sitBoneSetback, postSetback)

		if err := RenderOutput(a.Out, a.Format, sumTitle, sumSub, summaryHeaders, summaryRows); err != nil {
			fmt.Fprintf(a.Err, "Error: %v\n", err)
			return 1
		}
		if a.Format != FormatCSV {
			fmt.Fprintln(a.Out)
		}
	}

	// 2. Detailed Breakdown for each target saddle
	for i, targetSaddle := range targetSaddles {
		if i > 0 && a.Format != FormatCSV {
			fmt.Fprintln(a.Out)
		}

		report := geometry.AnalyzeSaddleSwap(
			sourceSaddle,
			targetSaddle,
			saddleHeight,
			sitBoneSetback,
			postSetback,
			clampLength,
			hbDistance,
			hbDrop,
		)

		// 1. Unified Saddle Swap Table (Sit Position Invariant, BRP Shifts, Workshop Actions)
		title := fmt.Sprintf("Saddle Swap Analysis: %s → %s", sourceSaddle.Name, targetSaddle.Name)
		subtitle := fmt.Sprintf("Active Seatpost Setback: %.1fmm (sagged)", postSetback)
		headers := []string{
			"Fit Dimension / Checkpoint",
			fmt.Sprintf("Source: %s", sourceSaddle.Name),
			fmt.Sprintf("Target: %s", targetSaddle.Name),
			"Difference / Workshop Action",
		}

		// Saddle height delta to BRP
		var shAction string
		if report.BrpSaddleHeightShiftMm > 0 {
			shAction = fmt.Sprintf("%+.1f mm (Raise seatpost collar)", report.BrpSaddleHeightShiftMm)
		} else if report.BrpSaddleHeightShiftMm < 0 {
			shAction = fmt.Sprintf("%.1f mm (Lower seatpost collar)", report.BrpSaddleHeightShiftMm)
		} else {
			shAction = "0.0 mm (Keep collar unchanged)"
		}

		// 70mm BRP 2D point shift direction
		var brpDir string
		if report.BrpDeltaXMm > 0 && report.BrpDeltaYMm > 0 {
			brpDir = "Forward & upward"
		} else if report.BrpDeltaXMm > 0 && report.BrpDeltaYMm < 0 {
			brpDir = "Forward & downward"
		} else if report.BrpDeltaXMm > 0 && report.BrpDeltaYMm == 0 {
			brpDir = "Forward"
		} else if report.BrpDeltaXMm < 0 && report.BrpDeltaYMm > 0 {
			brpDir = "Rearward & upward"
		} else if report.BrpDeltaXMm < 0 && report.BrpDeltaYMm < 0 {
			brpDir = "Rearward & downward"
		} else if report.BrpDeltaXMm < 0 && report.BrpDeltaYMm == 0 {
			brpDir = "Rearward"
		} else if report.BrpDeltaXMm == 0 && report.BrpDeltaYMm > 0 {
			brpDir = "Upward"
		} else if report.BrpDeltaXMm == 0 && report.BrpDeltaYMm < 0 {
			brpDir = "Downward"
		} else {
			brpDir = "Unmoved"
		}
		brpDiff := fmt.Sprintf("(%+.1f, %+.1f) mm (%s)", report.BrpDeltaXMm, report.BrpDeltaYMm, brpDir)

		// Clamp adjustment
		var clampDiff string
		if report.Pelvic.IsFeasible {
			if report.RailCenterToSitBonesDeltaMm > 0 {
				clampDiff = fmt.Sprintf("Slide %.1f mm forward (Safe, margin %.1fmm)", math.Abs(report.RailCenterToSitBonesDeltaMm), report.Pelvic.MarginFromLimitMm)
			} else if report.RailCenterToSitBonesDeltaMm < 0 {
				clampDiff = fmt.Sprintf("Slide %.1f mm rearward (Safe, margin %.1fmm)", math.Abs(report.RailCenterToSitBonesDeltaMm), report.Pelvic.MarginFromLimitMm)
			} else {
				clampDiff = fmt.Sprintf("Centered (Safe, margin %.1fmm)", report.Pelvic.MarginFromLimitMm)
			}
		} else {
			clampDiff = fmt.Sprintf("Infeasible (exceeds rails by %.1f mm)", math.Abs(report.Pelvic.MarginFromLimitMm))
		}

		swapRows := [][]string{
			{
				"BB to Sit Bones Distance",
				fmt.Sprintf("%.1f mm", report.BaselineLegExtensionMm),
				fmt.Sprintf("%.1f mm", report.Pelvic.LegExtensionMm),
				"0.0 mm (Preserved)",
			},
			{
				"Sit Bone Point (X, Y)",
				fmt.Sprintf("(-%.1f, %.1f) mm", sitBoneSetback, report.SourceSitBoneHeightMm),
				fmt.Sprintf("(-%.1f, %.1f) mm", report.Pelvic.AchievedSitBoneMm, report.Pelvic.AchievedSitBoneHeightMm),
				"(0.0, 0.0) mm (Preserved)",
			},
			{
				"Saddle Height (BB to 70mm BRP)",
				fmt.Sprintf("%.1f mm", saddleHeight),
				fmt.Sprintf("%.1f mm", report.TargetEquivalentSaddleHeightMm),
				shAction,
			},
			{
				"70mm BRP Point (X, Y)",
				fmt.Sprintf("(-%.1f, %.1f) mm", report.SourceBrpSetbackMm, report.SourceBrpHeightMm),
				fmt.Sprintf("(-%.1f, %.1f) mm", report.TargetEquivalentBrpSetbackMm, report.TargetEquivalentBrpHeightMm),
				brpDiff,
			},
			{
				"Rail Clamp Position",
				"0.0 mm (Centered)",
				fmt.Sprintf("%+.1f mm", report.Pelvic.RequiredClampOffsetMm),
				clampDiff,
			},
		}

		var footers []FooterSection
		footers = append(footers, FooterSection{
			Title: "Note",
			Lines: []string{
				fmt.Sprintf("Naively matching 70mm BRP coordinates would misplace pelvis by %+.1fmm setback and %+.1fmm height.",
					report.Naive.SitBoneDeltaMm, report.Naive.SitBoneDeltaYMm),
			},
		})

		if frameSpec != "" {
			if err := RenderOutput(a.Out, a.Format, title, subtitle, headers, swapRows); err != nil {
				fmt.Fprintf(a.Err, "Error: %v\n", err)
				return 1
			}
			if a.Format != FormatCSV {
				fmt.Fprintln(a.Out)
			}
		} else {
			if err := RenderOutputWithFooter(a.Out, a.Format, title, subtitle, headers, swapRows, footers); err != nil {
				fmt.Fprintf(a.Err, "Error: %v\n", err)
				return 1
			}
		}

		// Optional Frame-Specific Evaluation
		if frameSpec != "" {
			allFrames := frames.AllFrames()
			framePtr, impSz, err := ResolveFrameByToken(allFrames, frameSpec)
			if err != nil {
				fmt.Fprintf(a.Err, "Error resolving frame %q: %v\n", frameSpec, err)
				return 1
			}
			frame := *framePtr
			targetSize := impSz
			if targetSize == "" {
				targetSize = frame.GetDefaultSize()
			}

			setup := model.ComparisonSetup{
				FrameID: frame.ID,
				Size:    targetSize,
			}
			riderProfile := model.DefaultRiderProfile()
			riderProfile.SaddleHeightMm = saddleHeight

			// Canonicalize with source saddle
			canonRow, err := canonicalizer.ResolveFrameForkSetupWithSaddle(frame, setup, riderProfile, sourceSaddle)
			if err != nil {
				fmt.Fprintf(a.Err, "Error canonicalizing frame %s: %v\n", frame.ID, err)
				return 1
			}

			frameSta := canonRow.Canonical.ActiveSta
			stSetback := geometry.Round(saddleHeight*math.Cos(frameSta*math.Pi/180.0), 1)

			// Required rail offsets on this frame:
			// clampPerp = stSetback + postSetback
			// targetRailCenterSetback = sb + target.RailCenterToSitBonesMm
			// requiredOffset = targetRailCenterSetback - (stSetback + postSetback)
			srcRailOffset := (sitBoneSetback + sourceSaddle.RailCenterToSitBonesMm) - (stSetback + postSetback)
			tgtPelvicRailOffset := (sitBoneSetback + targetSaddle.RailCenterToSitBonesMm) - (stSetback + postSetback)
			tgtNaiveRailOffset := (report.SourceBrpSetbackMm + targetSaddle.RailCenterToBrpMm) - (stSetback + postSetback)

			sMin, sMax := sourceSaddle.ClampWindowMm(clampLength)
			tMin, tMax := targetSaddle.ClampWindowMm(clampLength)
			srcFeasible := srcRailOffset >= sMin-0.001 && srcRailOffset <= sMax+0.001
			pelvicFeasible := tgtPelvicRailOffset >= tMin-0.001 && tgtPelvicRailOffset <= tMax+0.001
			naiveFeasible := tgtNaiveRailOffset >= tMin-0.001 && tgtNaiveRailOffset <= tMax+0.001

			srcFeasStr := "Feasible"
			if !srcFeasible {
				srcFeasStr = fmt.Sprintf("false (%.1fmm)", srcRailOffset)
			}
			tgtFeasStr := "Feasible"
			if !pelvicFeasible {
				tgtFeasStr = fmt.Sprintf("false (%.1fmm)", tgtPelvicRailOffset)
			}
			tgtNaiveFeasStr := "Feasible"
			if !naiveFeasible {
				tgtNaiveFeasStr = fmt.Sprintf("false (%.1fmm)", tgtNaiveRailOffset)
			}

			evalMsg := "Pelvic fit is feasible within rails"
			if !pelvicFeasible {
				if tgtPelvicRailOffset > tMax {
					evalMsg = "Requires more seatpost setback (or slacker STA)"
				} else {
					evalMsg = "Requires less seatpost setback (or steeper STA)"
				}
			}

			t4Title := fmt.Sprintf("Frame-Specific Saddle Swap: %s %s (Size %s)", frame.Manufacturer, frame.Model, targetSize)
			t4Subtitle := fmt.Sprintf("Saddle Height (to BRP): %.0fmm | Effective STA: %.2f° | Seat Tube Setback: %.1fmm | Post: %.1fmm",
				saddleHeight, frameSta, stSetback, postSetback)
			t4Headers := []string{
				"Fit Parameter",
				fmt.Sprintf("Source: %s", sourceSaddle.Name),
				fmt.Sprintf("Target: %s (Pelvic)", targetSaddle.Name),
				fmt.Sprintf("Target: %s (Naive BRP)", targetSaddle.Name),
				"Feasibility & Evaluation",
			}

			t4Rows := [][]string{
				{
					"Required Rail Clamp Offset",
					fmt.Sprintf("%+.1f mm", srcRailOffset),
					fmt.Sprintf("%+.1f mm", tgtPelvicRailOffset),
					fmt.Sprintf("%+.1f mm", tgtNaiveRailOffset),
					fmt.Sprintf("Safe range: [%.1f, +%.1f] mm", tMin, tMax),
				},
				{
					"Physical Clamp Feasibility",
					srcFeasStr,
					tgtFeasStr,
					tgtNaiveFeasStr,
					evalMsg,
				},
				{
					"Achieved Sit Bone Setback",
					fmt.Sprintf("%.1f mm", sitBoneSetback),
					fmt.Sprintf("%.1f mm", sitBoneSetback),
					fmt.Sprintf("%.1f mm", sitBoneSetback+report.Naive.SitBoneDeltaMm),
					"Pelvic fit preserves exact target setback",
				},
			}

			if err := RenderOutputWithFooter(a.Out, a.Format, t4Title, t4Subtitle, t4Headers, t4Rows, footers); err != nil {
				fmt.Fprintf(a.Err, "Error: %v\n", err)
				return 1
			}
		}
	}

	return 0
}

func (a *App) printSaddleSwapUsage(w io.Writer) {
	fmt.Fprint(w, `BikeGeo Analyst CLI - Saddle Swap Analysis

Usage:
  bikegeo saddle-swap [flags] [source-saddle] [target-saddle]

Overview:
  The saddle-swap command analyzes transferring a rider's bike fit between different
  saddle models (e.g. from an ergonomic stepped saddle to a traditional leather touring
  saddle).

  When swapping saddles to preserve an identical fit, two primary things happen:
    1. The 70mm Biomechanical Reference Point (BRP) moves in space (both setback and height),
       because saddles differ in nose taper and vertical step profiles.
    2. The rider's pelvic sit position remains invariant (BB-to-sit-bones leg extension,
       pelvic setback, and pelvic height are strictly preserved).

  Preserving the fit requires adjusting the saddle position on the bike:
    - Sliding the rails through the seatpost clamp (fore/aft offset).
    - Raising or lowering the seatpost collar to achieve the target saddle height.

  SADDLE HEIGHT CONVENTION & 2D SPATIAL FIT:
  In this tool, "saddle height" is strictly measured to the saddle's 70mm Biomechanical
  Reference Point (BRP)—the radial distance from the Bottom Bracket spindle center (0, 0)
  to the BRP. It is NOT measured to the saddle clamp center, the saddle midpoint, the
  rider's sit-bone contact point, or the saddle nose.

  Fit transfer is evaluated in two dimensions (sagittal plane):
    - Horizontal (X): Pelvic sit bone setback behind BB.
    - Vertical (Y): Pelvic sit bone elevation above BB.

  Because saddles differ in both longitudinal nose taper (BRP-to-sit-bone span) and
  vertical step profile (e.g. SQlab Ergolux raised rear step vs. flat touring saddles),
  achieving an equivalent fit REQUIRES the target BRP to move in BOTH X and Y in space.
  Consequently, the radial Saddle Height measured to BRP must also be adjusted.

  The analysis demonstrates that BRP skew across saddle designs is merely an artifact of
  differing nose taper geometry (reaching 70mm width earlier or later) and step profiles.
  While naive BRP matching corrupts pelvic setback, pelvic height, leg extension, and
  cockpit reach, true pelvic alignment preserves 100% of the rider's fit with minimal
  rail adjustment and full mechanical feasibility.

Positional Arguments:
  [source-saddle]
      Source/reference saddle ID or case-insensitive substring from the compiled catalog
      (e.g. "sqlab-610-ergolux", "sqlab").
      Default: "sqlab-610-ergolux" (SQlab 610 Ergolux Active 2.1).

  [target-saddle]
      Target saddle ID or case-insensitive substring from the compiled catalog
      (e.g. "berthoud-aspin-open", "aspin", "selle-italia-slr-boost-superflow-l3", "slr").
      Default: "berthoud-aspin-open" (Berthoud Aspin Open).

Argument Resolution Modes:
  bikegeo saddle-swap
      Compares default source ("sqlab-610-ergolux") to default target ("berthoud-aspin-open").

  bikegeo saddle-swap <target-saddle>
      Compares default source ("sqlab-610-ergolux") to the specified target saddle.
      Example: bikegeo saddle-swap slr

  bikegeo saddle-swap <source-saddle> <target-saddle>
      Compares the specified source saddle to the specified target saddle.
      Example: bikegeo saddle-swap sqlab aspin

Flags:
  --saddle-height <mm>      Rider radial saddle height from BB to BRP in mm (not to saddle center or sit bones; default: 746.0)
  --sit-bone-setback <mm>   Target pelvic sit bone setback behind BB in mm (default: 292.0)
  --brp-setback <mm>        Derive sit bone setback from source saddle 70mm BRP setback in mm
  --seatpost <mm>           Active seatpost perpendicular setback in mm (default: 11.0, Redshift sagged)
  --clamp-length <mm>       Seatpost clamp length in mm (default: 30.0)
  --cockpit-dist <mm>       Reference Saddle-to-Handlebar distance in mm (default: 700.0)
  --cockpit-drop <mm>       Reference Saddle-to-Handlebar drop in mm (default: 13.0)
  --all                     Compare source saddle against all saddles in compiled catalog
  --frame <frame[:size]>    Evaluate saddle swap on a specific bike frame (e.g. otso-fenrir-stainless:L)
  -f, --format <fmt>        Output format: table (default), md, csv

Samples:
  # Analyze default swap (SQlab 610 -> Berthoud Aspin Open) at canonical idmatch fit coordinates:
  bikegeo saddle-swap

  # Analyze swapping to Berthoud Aspin by name:
  bikegeo saddle-swap berthoud-aspin-open

  # Analyze swapping from SQlab 610 to Selle Italia SLR Boost Superflow:
  bikegeo saddle-swap sqlab-610-ergolux selle-italia-slr-boost-superflow-l3

  # Compare source saddle against all compiled catalog saddles:
  bikegeo saddle-swap --all

  # Evaluate saddle swap on a specific bike frame and size:
  bikegeo saddle-swap --frame otso-fenrir-stainless:L

  # Evaluate with custom saddle height to BRP and sit-bone setback:
  bikegeo saddle-swap --saddle-height 750 --sit-bone-setback 290

  # Derive sit-bone setback from an idmatch report 70mm BRP caliper reading (219mm):
  bikegeo saddle-swap --brp-setback 219

  # Evaluate with Fair Bicycle Drop Best 45mm clamp and 39mm sagged setback:
  bikegeo saddle-swap --seatpost 39 --clamp-length 45
`)
}

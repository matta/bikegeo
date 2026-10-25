package cli

import (
	"flag"
	"fmt"
	"strings"

	"bikegeo/pkg/fit"
)

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSliceFlag) Set(val string) error {
	*s = append(*s, val)
	return nil
}

func (a *App) runFit(args []string) int {
	fs := flag.NewFlagSet("fit", flag.ContinueOnError)
	fs.SetOutput(a.Err)

	var configFlag string
	var detailFlag bool
	var tryFlags stringSliceFlag

	fs.StringVar(&configFlag, "config", "", "Specific frame configuration ID to analyze (defaults to baseline)")
	fs.BoolVar(&detailFlag, "detail", false, "Show detailed per-size residual table for each combination in --try sweeps")
	fs.Var(&tryFlags, "try", "Combinatorial trial parameter, e.g. --try fork.a2c=470,472 or --try XL:stack=655,660 (repeatable)")

	reordered := reorderFlagsForStdlib(fs, args, map[string]bool{"detail": true})
	if err := fs.Parse(reordered); err != nil {
		return 2
	}

	posArgs := fs.Args()
	if len(posArgs) < 1 {
		fmt.Fprintln(a.Err, "Usage: bikegeo fit <frame[:size]> [--try key=v1,v2,...] [--detail] [--config id]")
		return 2
	}

	store, err := a.openStore()
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}

	allFrames, err := store.GetAllFrames()
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}

	framePtr, impSz, err := ResolveFrameByToken(allFrames, posArgs[0])
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	frame := *framePtr

	targetSize := impSz
	if targetSize == "" {
		targetSize = frame.GetDefaultSize()
	}

	configID := configFlag
	if configID == "" {
		configID = "stock"
	}

	normFormat, _ := NormalizeFormat(a.Format)

	// Mode 1: Combinatorial Sweeps (--try)
	if len(tryFlags) > 0 {
		combos, err := fit.RunSweeps(frame, configID, targetSize, tryFlags)
		if err != nil {
			fmt.Fprintf(a.Err, "Error: %v\n", err)
			return 1
		}

		if normFormat == FormatJSON {
			if err := RenderOutput(a.Out, FormatJSON, "", "", nil, nil, combos); err != nil {
				fmt.Fprintf(a.Err, "Error: %v\n", err)
				return 1
			}
			return 0
		}

		title := fmt.Sprintf("Fit Sweep: %s %s (%d combinations)", frame.Manufacturer, frame.Model, len(combos))
		headers := []string{"Combo", "max|ΔA2C|", "max|ΔFC|", "max|ΔWB|", "Needed slop (a2c/fc/wb)", "Validate"}
		var rows [][]string

		for _, c := range combos {
			fcStr := "—"
			if c.FitResult != nil && c.FitResult.HasPublishedFc {
				fcStr = fmt.Sprintf("%.1f mm", c.MaxAbsDeltaFc)
			}
			wbStr := "—"
			if c.FitResult != nil && c.FitResult.HasPublishedWb {
				wbStr = fmt.Sprintf("%.1f mm", c.MaxAbsDeltaWb)
			}

			var slopParts []string
			slopParts = append(slopParts, fmt.Sprintf("%.1f", c.MinPassingSlopA2c))
			if c.FitResult != nil && c.FitResult.HasPublishedFc {
				slopParts = append(slopParts, fmt.Sprintf("%.1f", c.MinPassingSlopFc))
			} else {
				slopParts = append(slopParts, "—")
			}
			if c.FitResult != nil && c.FitResult.HasPublishedWb {
				slopParts = append(slopParts, fmt.Sprintf("%.1f", c.MinPassingSlopWb))
			} else {
				slopParts = append(slopParts, "—")
			}
			slopStr := strings.Join(slopParts, " / ")

			status := "FAIL"
			if c.IsValid {
				status = "PASS"
			}

			rows = append(rows, []string{
				c.Description,
				fmt.Sprintf("%.1f mm", c.MaxAbsDeltaA2c),
				fcStr,
				wbStr,
				slopStr,
				status,
			})
		}

		if err := RenderOutput(a.Out, a.Format, title, "", headers, rows, combos); err != nil {
			fmt.Fprintf(a.Err, "Error: %v\n", err)
			return 1
		}

		if detailFlag {
			for _, c := range combos {
				fmt.Fprintln(a.Out)
				comboTitle := fmt.Sprintf("Combination: %s", c.Description)
				a.renderResidualsTable(comboTitle, c.FitResult)
			}
		}
		return 0
	}

	// Mode 2: Single Configuration Fit Analysis
	fitRes, err := fit.AnalyzeFit(frame, configID)
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}

	if normFormat == FormatJSON {
		if err := RenderOutput(a.Out, FormatJSON, "", "", nil, nil, fitRes); err != nil {
			fmt.Fprintf(a.Err, "Error: %v\n", err)
			return 1
		}
		return 0
	}

	forkDesc := fmt.Sprintf("A2C %.0f mm, rake %.0f mm", frame.CadFork.A2cMm, frame.CadFork.RakeMm)
	if frame.CadFork.Fork != "" {
		forkDesc = fmt.Sprintf("%s (%s)", frame.CadFork.Fork, forkDesc)
	}

	spacerVal := frame.GetGeoHeadsetSpacerMm()
	sagVal := 0.0
	if frame.CadFork.SagPct != nil {
		sagVal = *frame.CadFork.SagPct
	}

	title := fmt.Sprintf(
		"%s %s — CAD Fork (%s, spacer %.1f mm, sag %.0f%%)",
		frame.Manufacturer, frame.Model, forkDesc, spacerVal, sagVal,
	)

	a.renderResidualsTable(title, fitRes)

	// Summary proofs & deductions
	fmt.Fprintf(a.Out, "\nBest-fit structural A2C (minimax): %.2f mm  (mean %.2f, spread %.2f)\n",
		fitRes.BestFitStructuralA2c, fitRes.MeanStructuralA2c, fitRes.SpreadStructuralA2c)
	fmt.Fprintf(a.Out, "  → implied spacer @ declared fork %.0f mm:   %.2f mm   (declared %.1f)\n",
		fitRes.DeclaredStaticForkA2c, fitRes.ImpliedSpacerMm, fitRes.DeclaredSpacerMm)
	fmt.Fprintf(a.Out, "  → implied static fork A2C @ declared spacer %.1f mm: %.2f mm  (declared %.0f)\n",
		fitRes.DeclaredSpacerMm, fitRes.ImpliedStaticForkA2c, fitRes.DeclaredStaticForkA2c)

	var slopReqParts []string
	slopReqParts = append(slopReqParts, fmt.Sprintf("a2c ±%.1f", fitRes.MinPassingSlopA2c))
	if fitRes.HasPublishedFc {
		slopReqParts = append(slopReqParts, fmt.Sprintf("fc ±%.1f", fitRes.MinPassingSlopFc))
	}
	if fitRes.HasPublishedWb {
		slopReqParts = append(slopReqParts, fmt.Sprintf("wb ±%.1f", fitRes.MinPassingSlopWb))
	}

	declaredSlop := frame.GetGeoSlopMm()
	var slopDeclParts []string
	slopDeclParts = append(slopDeclParts, fmt.Sprintf("a2c ±%.1f", declaredSlop.GetA2c()))
	if fitRes.HasPublishedFc {
		slopDeclParts = append(slopDeclParts, fmt.Sprintf("fc ±%.1f", declaredSlop.GetFrontCenter()))
	}
	if fitRes.HasPublishedWb {
		if declaredSlop.Wheelbase != nil {
			slopDeclParts = append(slopDeclParts, fmt.Sprintf("wb ±%.1f", *declaredSlop.Wheelbase))
		} else {
			slopDeclParts = append(slopDeclParts, "wb dynamic")
		}
	}

	fmt.Fprintf(a.Out, "Minimum passing slop: %s   (declared: %s)\n",
		strings.Join(slopReqParts, ", "), strings.Join(slopDeclParts, ", "))

	if declaredSlop.HasCustomSlop() {
		fmt.Fprintf(a.Out, "\n[NOTE] Frame defines custom geo_slop_mm overrides: %s\n", declaredSlop.String())
		if declaredSlop.Reason != "" {
			fmt.Fprintf(a.Out, "       Reason: %s\n", declaredSlop.Reason)
		}
	}

	statusBadge := "PASS"
	if !fitRes.IsValid {
		statusBadge = "FAIL"
	}
	fmt.Fprintf(a.Out, "ValidateFrame (declared values): %s\n", statusBadge)

	if !fitRes.IsValid && len(fitRes.FrameReport.Issues) > 0 {
		fmt.Fprintf(a.Out, "\nValidation Notices (%d):\n", len(fitRes.FrameReport.Issues))
		for _, iss := range fitRes.FrameReport.Issues {
			fmt.Fprintf(a.Out, "  [%s] %s: %s\n", strings.ToUpper(string(iss.Severity)), iss.Title, iss.Message)
		}
	}

	if !fitRes.IsValid || fitRes.MinPassingSlopA2c > 1.0 {
		candidates, err := fit.SolveFit(frame, configID, targetSize)
		if err == nil && len(candidates) > 0 {
			fmt.Fprintln(a.Out, "\nCandidate Physical Solutions (Geo slop is an absolute last resort):")
			var candHeaders = []string{"Physical Configuration", "max|ΔA2C|", "max|ΔWB|", "Valid?", "Suggested Command"}
			var candRows [][]string
			for _, c := range candidates {
				validStr := "FAIL"
				if c.IsValid {
					validStr = "PASS"
				}
				cmd := fmt.Sprintf("bikegeo set %s %s", frame.ID, strings.Join(c.Assignments, " "))
				candRows = append(candRows, []string{
					c.Description,
					fmt.Sprintf("%.1f mm", c.MaxAbsDeltaA2c),
					fmt.Sprintf("%.1f mm", c.MaxAbsDeltaWb),
					validStr,
					cmd,
				})
			}
			_ = RenderOutput(a.Out, a.Format, "", "", candHeaders, candRows, candidates)
		}
	}

	return 0
}

func (a *App) renderResidualsTable(title string, fitRes *fit.FitResult) {
	headers := []string{"Size", "HTA", "Implied Struct A2C", "Δ vs Best", "FC pub", "FC calc", "ΔFC", "WB pub", "WB calc", "ΔWB"}
	var rows [][]string

	for _, r := range fitRes.Residuals {
		fcPub := "—"
		fcCalc := "—"
		deltaFc := "—"
		if r.FcPub > 0 {
			fcPub = fmt.Sprintf("%.1f", r.FcPub)
			fcCalc = fmt.Sprintf("%.1f", r.FcCalc)
			if r.DeltaFc != nil {
				deltaFc = fmt.Sprintf("%+.1f", *r.DeltaFc)
			}
		}

		wbPub := "—"
		wbCalc := "—"
		deltaWb := "—"
		if r.WbPub > 0 && r.ChainstayLength > 0 {
			wbPub = fmt.Sprintf("%.1f", r.WbPub)
			wbCalc = fmt.Sprintf("%.1f", r.WbCalc)
			if r.DeltaWb != nil {
				deltaWb = fmt.Sprintf("%+.1f", *r.DeltaWb)
			}
		}

		rows = append(rows, []string{
			r.Size,
			fmt.Sprintf("%.1f°", r.HtaDeg),
			fmt.Sprintf("%.1f", r.ImpliedStructuralA2c),
			fmt.Sprintf("%+.2f", r.DeltaVsBest),
			fcPub,
			fcCalc,
			deltaFc,
			wbPub,
			wbCalc,
			deltaWb,
		})
	}

	_ = RenderOutput(a.Out, a.Format, title, "", headers, rows, fitRes)
}

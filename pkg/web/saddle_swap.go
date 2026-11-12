package web

import (
	"fmt"
	"math"
	"net/url"
	"strings"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/catalog"
	"bikegeo/pkg/compare"
	"bikegeo/pkg/frames"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
	"bikegeo/pkg/report"
)

// defaultSwapTarget is the target used when only a source saddle is chosen.
const defaultSwapTarget = "berthoud-aspin-open"

func (db *docb) saddleSwap(v url.Values) int {
	fl := &floats{v: v}
	saddleHeight := fl.get("saddle-height", 746.0)
	sitBoneSetback := fl.get("sit-bone-setback", 292.0)
	brpSetback := fl.get("brp-setback", 0.0)
	postSetback := fl.get("seatpost", model.DefaultSeatpostSetbackMm)
	clampLength := fl.get("clamp-length", model.DefaultSeatpostClampLengthMm)
	hbDistance := fl.get("cockpit-dist", 700.0)
	hbDrop := fl.get("cockpit-drop", 13.0)
	if fl.err != nil {
		db.fail("%v", fl.err)
		return 1
	}
	compareAll := isTrue(v.Get("all"))
	frameSpec := strings.TrimSpace(v.Get("frame"))
	var posArgs []string
	from, to := strings.TrimSpace(v.Get("from")), strings.TrimSpace(v.Get("to"))
	if to != "" && from == "" {
		from = catalog.DefaultSaddle().ID
	}
	if from != "" && to == "" && !compareAll {
		to = defaultSwapTarget
	}
	if from != "" {
		posArgs = append(posArgs, from)
	}
	if to != "" {
		posArgs = append(posArgs, to)
	}
	sourceSaddle := catalog.DefaultSaddle()
	var targetSaddles []model.SaddleSpec

	if compareAll {
		if len(posArgs) > 0 {
			if s, ok := catalog.GetSaddle(posArgs[0]); ok {
				sourceSaddle = s
			} else {
				db.fail("Error: unrecognized source saddle %q", posArgs[0])
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
			db.fail("Error: unrecognized source saddle %q", posArgs[0])
			return 1
		}
		if s, ok := catalog.GetSaddle(posArgs[1]); ok {
			targetSaddles = []model.SaddleSpec{s}
		} else {
			db.fail("Error: unrecognized target saddle %q", posArgs[1])
			return 1
		}
	} else if len(posArgs) == 1 {
		if s, ok := catalog.GetSaddle(posArgs[0]); ok {
			targetSaddles = []model.SaddleSpec{s}
		} else {
			db.fail("Error: unrecognized target saddle %q", posArgs[0])
			return 1
		}
	} else {
		// Default: SQlab 610 Ergolux -> Berthoud Aspin Open
		if aspin, ok := catalog.GetSaddle("berthoud-aspin-open"); ok {
			targetSaddles = []model.SaddleSpec{aspin}
		} else {
			db.fail("Error: default target saddle berthoud-aspin-open not found")
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

		if err := db.render(sumTitle, sumSub, summaryHeaders, summaryRows); err != nil {
			db.fail("Error: %v", err)
			return 1
		}
	}

	// 2. Detailed Breakdown for each target saddle
	for _, targetSaddle := range targetSaddles {

		swap := geometry.AnalyzeSaddleSwap(
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
		if swap.BrpSaddleHeightShiftMm > 0 {
			shAction = fmt.Sprintf("%+.1f mm (Raise seatpost collar)", swap.BrpSaddleHeightShiftMm)
		} else if swap.BrpSaddleHeightShiftMm < 0 {
			shAction = fmt.Sprintf("%.1f mm (Lower seatpost collar)", swap.BrpSaddleHeightShiftMm)
		} else {
			shAction = "0.0 mm (Keep collar unchanged)"
		}

		// 70mm BRP 2D point shift direction
		var brpDir string
		if swap.BrpDeltaXMm > 0 && swap.BrpDeltaYMm > 0 {
			brpDir = "Forward & upward"
		} else if swap.BrpDeltaXMm > 0 && swap.BrpDeltaYMm < 0 {
			brpDir = "Forward & downward"
		} else if swap.BrpDeltaXMm > 0 && swap.BrpDeltaYMm == 0 {
			brpDir = "Forward"
		} else if swap.BrpDeltaXMm < 0 && swap.BrpDeltaYMm > 0 {
			brpDir = "Rearward & upward"
		} else if swap.BrpDeltaXMm < 0 && swap.BrpDeltaYMm < 0 {
			brpDir = "Rearward & downward"
		} else if swap.BrpDeltaXMm < 0 && swap.BrpDeltaYMm == 0 {
			brpDir = "Rearward"
		} else if swap.BrpDeltaXMm == 0 && swap.BrpDeltaYMm > 0 {
			brpDir = "Upward"
		} else if swap.BrpDeltaXMm == 0 && swap.BrpDeltaYMm < 0 {
			brpDir = "Downward"
		} else {
			brpDir = "Unmoved"
		}
		brpDiff := fmt.Sprintf("(%+.1f, %+.1f) mm (%s)", swap.BrpDeltaXMm, swap.BrpDeltaYMm, brpDir)

		// Clamp adjustment
		var clampDiff string
		if swap.Pelvic.IsFeasible {
			if swap.RailCenterToSitBonesDeltaMm > 0 {
				clampDiff = fmt.Sprintf("Slide %.1f mm forward (Safe, margin %.1fmm)", math.Abs(swap.RailCenterToSitBonesDeltaMm), swap.Pelvic.MarginFromLimitMm)
			} else if swap.RailCenterToSitBonesDeltaMm < 0 {
				clampDiff = fmt.Sprintf("Slide %.1f mm rearward (Safe, margin %.1fmm)", math.Abs(swap.RailCenterToSitBonesDeltaMm), swap.Pelvic.MarginFromLimitMm)
			} else {
				clampDiff = fmt.Sprintf("Centered (Safe, margin %.1fmm)", swap.Pelvic.MarginFromLimitMm)
			}
		} else {
			clampDiff = fmt.Sprintf("Infeasible (exceeds rails by %.1f mm)", math.Abs(swap.Pelvic.MarginFromLimitMm))
		}

		swapRows := [][]string{
			{
				"BB to Sit Bones Distance",
				fmt.Sprintf("%.1f mm", swap.BaselineLegExtensionMm),
				fmt.Sprintf("%.1f mm", swap.Pelvic.LegExtensionMm),
				"0.0 mm (Preserved)",
			},
			{
				"Sit Bone Point (X, Y)",
				fmt.Sprintf("(-%.1f, %.1f) mm", sitBoneSetback, swap.SourceSitBoneHeightMm),
				fmt.Sprintf("(-%.1f, %.1f) mm", swap.Pelvic.AchievedSitBoneMm, swap.Pelvic.AchievedSitBoneHeightMm),
				"(0.0, 0.0) mm (Preserved)",
			},
			{
				"Saddle Height (BB to 70mm BRP)",
				fmt.Sprintf("%.1f mm", saddleHeight),
				fmt.Sprintf("%.1f mm", swap.TargetEquivalentSaddleHeightMm),
				shAction,
			},
			{
				"70mm BRP Point (X, Y)",
				fmt.Sprintf("(-%.1f, %.1f) mm", swap.SourceBrpSetbackMm, swap.SourceBrpHeightMm),
				fmt.Sprintf("(-%.1f, %.1f) mm", swap.TargetEquivalentBrpSetbackMm, swap.TargetEquivalentBrpHeightMm),
				brpDiff,
			},
			{
				"Rail Clamp Position",
				"0.0 mm (Centered)",
				fmt.Sprintf("%+.1f mm", swap.Pelvic.RequiredClampOffsetMm),
				clampDiff,
			},
		}

		var footers []report.Footer
		footers = append(footers, report.Footer{
			Title: "Note",
			Lines: []string{
				fmt.Sprintf("Naively matching 70mm BRP coordinates would misplace pelvis by %+.1fmm setback and %+.1fmm height.",
					swap.Naive.SitBoneDeltaMm, swap.Naive.SitBoneDeltaYMm),
			},
		})

		if frameSpec != "" {
			if err := db.render(title, subtitle, headers, swapRows); err != nil {
				db.fail("Error: %v", err)
				return 1
			}
		} else {
			if err := db.renderFooters(title, subtitle, headers, swapRows, footers); err != nil {
				db.fail("Error: %v", err)
				return 1
			}
		}

		// Optional Frame-Specific Evaluation
		if frameSpec != "" {
			allFrames := frames.AllFrames()
			framePtr, impSz, err := compare.ResolveFrameByToken(allFrames, frameSpec)
			if err != nil {
				db.fail("Error resolving frame %q: %v", frameSpec, err)
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
			riderProfile := catalog.DefaultRiderProfile()
			riderProfile.SaddleHeightMm = saddleHeight

			// Canonicalize with source saddle
			canonRow, err := canonicalizer.ResolveFrameForkSetupWithSaddle(frame, setup, riderProfile, sourceSaddle)
			if err != nil {
				db.fail("Error canonicalizing frame %s: %v", frame.ID, err)
				return 1
			}

			frameSta := canonRow.Canonical.ActiveSta
			stSetback := geometry.Round(saddleHeight*math.Cos(frameSta*math.Pi/180.0), 1)
			tgtStSetback := geometry.Round(swap.TargetEquivalentSaddleHeightMm*math.Cos(frameSta*math.Pi/180.0), 1)

			// Required rail offsets on this frame:
			// clampPerp = stSetback + postSetback
			// targetRailCenterSetback = sb + target.RailCenterToSitBonesMm
			// requiredOffset = targetRailCenterSetback - (stSetback + postSetback)
			srcRailOffset := (sitBoneSetback + sourceSaddle.RailCenterToSitBonesMm) - (stSetback + postSetback)
			tgtPelvicRailOffset := (sitBoneSetback + targetSaddle.RailCenterToSitBonesMm) - (tgtStSetback + postSetback)
			tgtNaiveRailOffset := (swap.SourceBrpSetbackMm + targetSaddle.RailCenterToBrpMm) - (stSetback + postSetback)

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
					fmt.Sprintf("%.1f mm", sitBoneSetback+swap.Naive.SitBoneDeltaMm),
					"Pelvic fit preserves exact target setback",
				},
			}

			if err := db.renderFooters(t4Title, t4Subtitle, t4Headers, t4Rows, footers); err != nil {
				db.fail("Error: %v", err)
				return 1
			}
		}
	}

	return 0
}

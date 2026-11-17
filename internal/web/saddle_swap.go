package web

import (
	"fmt"
	"math"
	"net/url"
	"strings"

	"bikegeo/internal/canonicalizer"
	"bikegeo/internal/catalog"
	"bikegeo/internal/compare"
	"bikegeo/internal/frames"
	"bikegeo/internal/geom2d"
	"bikegeo/internal/geometry"
	"bikegeo/internal/ideal"
	"bikegeo/internal/model"
	"bikegeo/internal/report"
	"bikegeo/internal/validator"
)

// defaultSwapTarget is the target used when only a source saddle is chosen.
const defaultSwapTarget = "berthoud-aspin-open"

func clampPositionStatus(s model.SaddleSpec, footprint, offset float64) string {
	lo, hi, ok := s.ClampWindowMm(footprint)
	if !ok {
		return "Clamp exceeds assumed rail span"
	}
	if math.IsNaN(offset) || math.IsInf(offset, 0) {
		return "No modeled clamp position"
	}
	if !s.ClampFitsAt(footprint, offset) {
		return fmt.Sprintf("Outside assumed window [%+.1f, %+.1f] mm", lo, hi)
	}
	return fmt.Sprintf("Within assumed window [%+.1f, %+.1f] mm", lo, hi)
}

func (db *docb) saddleSwap(v url.Values) int {
	fl := &floats{v: v}
	saddleHeight := fl.get("saddle-height", model.DefaultIdmatchSaddleHeightMm)
	sitBoneSetback := fl.get("sit-bone-setback", model.DefaultTargetSitBoneSetbackMm())
	brpSetback := fl.get("brp-setback", 0.0)
	postSetback := fl.get("seatpost", model.DefaultSeatpostSetbackMm)
	clampLength := fl.get("clamp-length", model.DefaultSeatpostClampLengthMm)
	if fl.err != nil {
		db.fail("%v", fl.err)
		return 1
	}
	hasSitBone := strings.TrimSpace(v.Get("sit-bone-setback")) != ""
	hasBrp := strings.TrimSpace(v.Get("brp-setback")) != ""
	if hasSitBone && hasBrp {
		db.fail("Error: cannot specify both brp-setback and sit-bone-setback")
		return 1
	}
	if !(saddleHeight > 0) || math.IsInf(saddleHeight, 0) {
		db.fail("Error: saddle-height must be positive (got %g)", saddleHeight)
		return 1
	}
	if hasSitBone && (!(sitBoneSetback > 0) || math.IsInf(sitBoneSetback, 0)) {
		db.fail("Error: sit-bone-setback must be positive (got %g)", sitBoneSetback)
		return 1
	}
	if hasBrp && (!(brpSetback > 0) || math.IsInf(brpSetback, 0)) {
		db.fail("Error: brp-setback must be positive (got %g)", brpSetback)
		return 1
	}
	if math.IsNaN(postSetback) || math.IsInf(postSetback, 0) || postSetback < 0 {
		db.fail("Error: seatpost setback must be non-negative (got %g)", postSetback)
		return 1
	}
	if !(clampLength > 0) || math.IsInf(clampLength, 0) {
		db.fail("Error: clamp-length must be positive (got %g)", clampLength)
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

	if hasBrp {
		sitBoneSetback = sourceSaddle.SitBoneSetbackFromBrp(brpSetback)
		if !(sitBoneSetback > 0) {
			db.fail("Error: brp-setback %.1fmm yields non-positive sit-bone setback %.1fmm", brpSetback, sitBoneSetback)
			return 1
		}
	}

	hbDistance, riseAboveSit, _ := ideal.IdmatchProfile().CockpitTarget(sourceSaddle)
	hbDrop := riseAboveSit + sourceSaddle.SitBonePointFromBrp(geom2d.Origin()).Y

	// Catalog overview when every target saddle is requested.
	if compareAll && len(targetSaddles) > 0 {
		summaryHeaders := []string{
			"Target Saddle",
			"Catalog Rail Span",
			"BRP to SIT (rearward)",
			"Step Ht (ΔY)",
			"Equivalent BRP (X, Y)",
			"Target Saddle Height (BRP)",
			"Local Rail-to-SIT Δ",
			"Required Clamp Station",
			"Centered STA Δ",
			"30mm Clamp",
			"45mm Clamp",
			fmt.Sprintf("Selected %.1f mm Footprint", clampLength),
		}
		var summaryRows [][]string

		for _, tgt := range targetSaddles {
			rep, err := geometry.AnalyzeSaddleSwap(
				sourceSaddle,
				tgt,
				saddleHeight,
				sitBoneSetback,
				postSetback,
				clampLength,
				hbDistance,
				hbDrop,
			)
			if err != nil {
				db.fail("Error: %v", err)
				return 1
			}

			offset := rep.PreserveSIT.RequiredClampOffsetMm

			summaryRows = append(summaryRows, []string{
				tgt.Name,
				fmt.Sprintf("%.0f mm", tgt.RailLengthMm),
				fmt.Sprintf("%.1f mm", tgt.DeltaBrpToSitBonesMm()),
				fmt.Sprintf("%.1f mm (%+.1f mm)", tgt.SitBonesElevationMm, rep.StepElevationDeltaMm),
				fmt.Sprintf("(-%.1f, %.1f) mm (%+.1f, %+.1f)", rep.TargetEquivalentBrpSetbackMm, rep.TargetEquivalentBrpHeightMm, rep.BrpDeltaXMm, rep.BrpDeltaYMm),
				fmt.Sprintf("%.1f mm (%+.1f mm)", rep.TargetEquivalentSaddleHeightMm, rep.BrpSaddleHeightShiftMm),
				fmt.Sprintf("%+.1f mm", rep.RailCenterToSitBonesDeltaMm),
				fmt.Sprintf("%+.1f mm", offset),
				fmt.Sprintf("%+.2f°", rep.CenteredStaDeltaDeg),
				clampPositionStatus(tgt, model.DefaultSeatpostClampLengthMm, offset),
				clampPositionStatus(tgt, model.DropBestClampLengthMm, offset),
				clampPositionStatus(tgt, clampLength, offset),
			})
		}

		sumTitle := fmt.Sprintf("Saddle Swap Catalog Overview (Baseline: %s)", sourceSaddle.Name)
		sumSub := fmt.Sprintf("Source Saddle Height (to BRP): %.0fmm | SIT Setback: %.0fmm behind BB | Post Setback: %.1fmm",
			saddleHeight, sitBoneSetback, postSetback)

		if err := db.render(sumTitle, sumSub, summaryHeaders, summaryRows); err != nil {
			db.fail("Error: %v", err)
			return 1
		}
	}

	// 2. Detailed Breakdown for each target saddle
	for _, targetSaddle := range targetSaddles {

		swap, err := geometry.AnalyzeSaddleSwap(
			sourceSaddle,
			targetSaddle,
			saddleHeight,
			sitBoneSetback,
			postSetback,
			clampLength,
			hbDistance,
			hbDrop,
		)
		if err != nil {
			db.fail("Error: %v", err)
			return 1
		}

		// Show the modeled placement with SIT held fixed.
		title := fmt.Sprintf("Saddle Swap Analysis: %s → %s", sourceSaddle.Name, targetSaddle.Name)
		subtitle := fmt.Sprintf("Assumed seat-perpendicular setback: %.1f mm", postSetback)
		headers := []string{
			"Fit Dimension / Checkpoint",
			fmt.Sprintf("Source: %s", sourceSaddle.Name),
			fmt.Sprintf("Target: %s", targetSaddle.Name),
			"Modeled Change",
		}

		swapRows := [][]string{
			{
				"BB-to-SIT Distance (geometric proxy)",
				fmt.Sprintf("%.1f mm", swap.BaselineBBToSitDistanceMm),
				fmt.Sprintf("%.1f mm", swap.PreserveSIT.BBToSitDistanceMm),
				"0.0 mm (Preserved)",
			},
			{
				"Modeled SIT Point (X, Y)",
				fmt.Sprintf("(-%.1f, %.1f) mm", sitBoneSetback, swap.SourceSitBoneHeightMm),
				fmt.Sprintf("(-%.1f, %.1f) mm", swap.PreserveSIT.AchievedSitBoneMm, swap.PreserveSIT.AchievedSitBoneHeightMm),
				"(0.0, 0.0) mm (Preserved)",
			},
			{
				"Saddle Height (BB to 70mm BRP)",
				fmt.Sprintf("%.1f mm", saddleHeight),
				fmt.Sprintf("%.1f mm", swap.TargetEquivalentSaddleHeightMm),
				fmt.Sprintf("%+.1f mm of radial BRP height", swap.BrpSaddleHeightShiftMm),
			},
			{
				"70mm BRP Point (X, Y)",
				fmt.Sprintf("(-%.1f, %.1f) mm", swap.SourceBrpSetbackMm, swap.SourceBrpHeightMm),
				fmt.Sprintf("(-%.1f, %.1f) mm", swap.TargetEquivalentBrpSetbackMm, swap.TargetEquivalentBrpHeightMm),
				fmt.Sprintf("(%+.1f, %+.1f) mm", swap.BrpDeltaXMm, swap.BrpDeltaYMm),
			},
			{
				"BRP-to-Cockpit Distance",
				fmt.Sprintf("%.1f mm", hbDistance),
				fmt.Sprintf("%.1f mm", swap.TargetBrpToCockpitMm),
				"Same modeled cockpit point",
			},
			{
				"Rail Clamp Position",
				fmt.Sprintf("0.0 mm (%s)", clampPositionStatus(sourceSaddle, clampLength, 0)),
				fmt.Sprintf("%+.1f mm", swap.PreserveSIT.RequiredClampOffsetMm),
				clampPositionStatus(targetSaddle, clampLength, swap.PreserveSIT.RequiredClampOffsetMm),
			},
		}

		var footers []report.Footer
		footers = append(footers, report.Footer{
			Title: "How to read the swap",
			Lines: []string{
				clampAssumption,
				fmt.Sprintf("Matching the source BRP alone would change the modeled SIT setback by %+.1f mm and its height by %+.1f mm.",
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
			canonRow, err := canonicalizer.ResolveFrameForkSetupWithSaddle(frame, setup, riderProfile, sourceSaddle, validator.Catalog{
				Forks: catalog.CommonForkCatalog,
				Stems: catalog.CommonStemCatalog,
			})
			if err != nil {
				db.fail("Error canonicalizing frame %s: %v", frame.ID, err)
				return 1
			}

			frameSize, err := geometry.FrameSizeFromBikeGeometry(canonRow.Bike)
			if err != nil {
				db.fail("%v", err)
				return 1
			}
			pitch := canonRow.Canonical.DeltaAngleDeg
			frameSta := frameSize.EffectiveStaDegAt(saddleHeight) - pitch
			stSetback := saddleHeight * frameSta.Angle().Cos()
			srcBrpSetback := sourceSaddle.BrpSetbackFromSitBones(sitBoneSetback)
			srcRailOffset := sourceSaddle.RailOffsetForBrpAtSta(frameSta, srcBrpSetback, saddleHeight, postSetback)
			reference, err := geometry.SolveBRPPoint(sourceSaddle, frameSta, saddleHeight, postSetback, srcRailOffset)
			if err != nil {
				db.fail("%v", err)
				return 1
			}
			targetHeight := targetSaddle.BrpPointFromSitBones(reference.SitBonePoint()).DistanceToOrigin()
			targetSTA := frameSize.EffectiveStaDegAt(targetHeight) - pitch
			translated, err := geometry.TranslateSaddleFit(reference, targetSaddle, targetSTA, clampLength)
			if err != nil {
				db.fail("%v", err)
				return 1
			}
			tgtSupportRailOffset := translated.TargetClampOffset
			tgtBrpRailOffset := targetSaddle.RailOffsetForBrpAtSta(frameSta, srcBrpSetback, saddleHeight, postSetback)

			t4Title := fmt.Sprintf("Frame-Specific Saddle Swap: %s %s (Size %s)", frame.Manufacturer, frame.Model, targetSize)
			t4Subtitle := fmt.Sprintf("Saddle Height (to BRP): %.0fmm | Effective STA: %.2f° | Seat Tube Setback: %.1fmm | Post: %.1fmm",
				saddleHeight, frameSta, stSetback, postSetback)
			t4Headers := []string{
				"Fit Parameter",
				fmt.Sprintf("Source: %s", sourceSaddle.Name),
				fmt.Sprintf("Target: %s (match SIT)", targetSaddle.Name),
				fmt.Sprintf("Target: %s (match BRP)", targetSaddle.Name),
				"Feasibility & Evaluation",
			}

			t4Rows := [][]string{
				{
					"Required Rail Clamp Offset",
					fmt.Sprintf("%+.1f mm", srcRailOffset),
					fmt.Sprintf("%+.1f mm", tgtSupportRailOffset),
					fmt.Sprintf("%+.1f mm", tgtBrpRailOffset),
					"Positive toward the nose",
				},
				{
					"Modeled Clamp Fit",
					clampPositionStatus(sourceSaddle, clampLength, srcRailOffset),
					clampPositionStatus(targetSaddle, clampLength, tgtSupportRailOffset),
					clampPositionStatus(targetSaddle, clampLength, tgtBrpRailOffset),
					fmt.Sprintf("Assumed %.1f mm footprint", clampLength),
				},
				{
					"Modeled SIT Setback",
					fmt.Sprintf("%.1f mm", sitBoneSetback),
					fmt.Sprintf("%.1f mm", sitBoneSetback),
					fmt.Sprintf("%.1f mm", -targetSaddle.SitBonePointFromBrp(reference.BrpPoint()).X),
					"Matching SIT keeps the support point fixed",
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

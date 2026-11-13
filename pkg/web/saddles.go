package web

import (
	"fmt"
	"math"
	"net/url"
	"strings"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
	"bikegeo/pkg/report"
)

func (db *docb) saddles(v url.Values) int {
	fl := &floats{v: v}
	saddleHeight := fl.get("saddle-height", 746.0)
	sitBoneSetback := fl.get("sit-bone-setback", 292.0)
	if fl.err != nil {
		db.fail("%v", fl.err)
		return 1
	}
	if !(saddleHeight > 0) || math.IsInf(saddleHeight, 0) {
		db.fail("Error: saddle-height must be positive (got %g)", saddleHeight)
		return 1
	}
	if !(sitBoneSetback > 0) || math.IsInf(sitBoneSetback, 0) {
		db.fail("Error: sit-bone-setback must be positive (got %g)", sitBoneSetback)
		return 1
	}
	var posArgs []string
	if id := strings.TrimSpace(v.Get("id")); id != "" {
		posArgs = []string{id}
	}
	var selectedSaddle model.SaddleSpec
	if len(posArgs) > 0 {
		query := strings.TrimSpace(posArgs[0])
		if s, ok := catalog.GetSaddle(query); ok {
			selectedSaddle = s
		} else {
			db.fail("Error: unrecognized saddle %q", query)
			return 1
		}
	} else {
		selectedSaddle = catalog.DefaultSaddle()
	}

	saddles := catalog.AllSaddles()
	catalogSaddles := saddles
	if len(posArgs) > 0 {
		catalogSaddles = []model.SaddleSpec{selectedSaddle}
	}
	headers := []string{
		"Saddle ID",
		"Name",
		"Rail Length",
		"Rail -> BRP (70mm)",
		"Rail -> Sit Bones",
		"BRP -> Sit Bones",
		"Step Elevation",
	}
	var rows [][]string

	for _, s := range catalogSaddles {
		stepStr := "0.0 mm"
		if s.SitBonesElevationMm > 0 {
			stepStr = fmt.Sprintf("+%.1f mm", s.SitBonesElevationMm)
		}
		rows = append(rows, []string{
			s.ID,
			s.Name,
			fmt.Sprintf("%.0f mm", s.RailLengthMm),
			fmt.Sprintf("+%.1f mm", s.RailCenterToBrpMm),
			fmt.Sprintf("%.1f mm", s.RailCenterToSitBonesMm),
			fmt.Sprintf("%.1f mm", s.DeltaBrpToSitBonesMm()),
			stepStr,
		})
	}

	title := fmt.Sprintf("Canonical Saddle Catalog (%d)", len(catalogSaddles))
	subtitle := "Origin (0.0mm) is the midpoint of the straight rail clamping zone; (+) forward toward nose, (-) rearward toward tail"

	var footers []report.Footer

	// 1. Cross-Saddle Translation Section (SQlab 610 vs Berthoud Aspin & Selle Italia SLR Boost)
	sqlab, hasSQ := catalog.GetSaddle("sqlab-610-ergolux")
	aspin, hasAspin := catalog.GetSaddle("berthoud-aspin-open")
	slr, hasSLR := catalog.GetSaddle("selle-italia-slr-boost-superflow-l3")
	var refBrpSetback float64
	if hasSQ {
		refBrpSetback = sqlab.BrpSetbackFromSitBones(sitBoneSetback)
	}
	if len(posArgs) == 0 && hasSQ && hasAspin && hasSLR {
		transAspinDropBest := geometry.TranslateSaddleFit(sqlab, -15.0, aspin, model.DropBestClampLengthMm)
		transAspinStd := geometry.TranslateSaddleFit(sqlab, 0.0, aspin, model.DefaultSeatpostClampLengthMm)
		transSlrDropBest := geometry.TranslateSaddleFit(sqlab, -15.0, slr, model.DropBestClampLengthMm)
		transSlrStd := geometry.TranslateSaddleFit(sqlab, 0.0, slr, model.DefaultSeatpostClampLengthMm)

		shSQ11 := sqlab.TranslateSaddleHeightFromRef(sqlab, saddleHeight, refBrpSetback)
		shAspin11 := aspin.TranslateSaddleHeightFromRef(sqlab, saddleHeight, refBrpSetback)
		shSLR11 := slr.TranslateSaddleHeightFromRef(sqlab, saddleHeight, refBrpSetback)
		resSQ11 := geometry.CalculateIdealSeatTubeAngle(sqlab, sitBoneSetback, shSQ11, model.DefaultSeatpostSetbackMm, "11mm", model.DefaultSeatpostClampLengthMm)
		resAspin11 := geometry.CalculateIdealSeatTubeAngle(aspin, sitBoneSetback, shAspin11, model.DefaultSeatpostSetbackMm, "11mm", model.DefaultSeatpostClampLengthMm)
		resSLR11 := geometry.CalculateIdealSeatTubeAngle(slr, sitBoneSetback, shSLR11, model.DefaultSeatpostSetbackMm, "11mm", model.DefaultSeatpostClampLengthMm)
		deltaAspin11 := resAspin11.IdealStaDeg - resSQ11.IdealStaDeg
		deltaSlr11 := resSLR11.IdealStaDeg - resSQ11.IdealStaDeg
		aspinDir := "slackening STA by"
		if deltaAspin11 >= 0 {
			aspinDir = "steepening STA by"
		}
		slrDir := "steepening STA by"
		if deltaSlr11 < 0 {
			slrDir = "slackening STA by"
		}

		footers = append(footers, report.Footer{
			Title: "Cross-Saddle Fit Translation: SQlab 610 Ergolux vs. Catalog Saddles",
			Lines: []string{
				fmt.Sprintf("Pelvic Fit Equivalence (Aspin): Rail-to-sit-bone delta is only %.1f mm (Aspin %.1fmm vs SQlab %.1fmm), %s ~%.2f° at %.0fmm SH / %.0fmm sit-bone setback (11mm post).",
					transAspinDropBest.SitBoneDeltaMm, aspin.RailCenterToSitBonesMm, sqlab.RailCenterToSitBonesMm, aspinDir, deltaAspin11.Abs(), saddleHeight, sitBoneSetback),
				fmt.Sprintf("Pelvic Fit Equivalence (SLR Boost): Sit bones sit %.1f mm further rearward (SLR %.1fmm vs SQlab %.1fmm), %s ~%.2f° at %.0fmm SH / %.0fmm sit-bone setback (11mm post).",
					math.Abs(transSlrDropBest.SitBoneDeltaMm), slr.RailCenterToSitBonesMm, sqlab.RailCenterToSitBonesMm, slrDir, deltaSlr11.Abs(), saddleHeight, sitBoneSetback),
				fmt.Sprintf("BRP Caliper Discrepancy: BRP lines vary widely due to nose taper (Aspin %.0fmm vs SQlab %.0fmm vs SLR Boost %.0fmm from sit bones), proving raw BRP misleads without resolving sit bones.",
					aspin.DeltaBrpToSitBonesMm(), sqlab.DeltaBrpToSitBonesMm(), slr.DeltaBrpToSitBonesMm()),
				fmt.Sprintf("Clamp Translation to Aspin: Clamping SQlab at -15.0mm translates to %+.1fmm on Aspin (window [%.1f, %+.1f] mm; feasible: %t); dead-center translates to %+.1fmm (window [%.1f, %+.1f] mm).",
					transAspinDropBest.TargetClampOffset, transAspinDropBest.TargetClampMin, transAspinDropBest.TargetClampMax, transAspinDropBest.IsFeasible,
					transAspinStd.TargetClampOffset, transAspinStd.TargetClampMin, transAspinStd.TargetClampMax),
				fmt.Sprintf("Clamp Translation to SLR Boost: Clamping SQlab at dead-center (0.0mm) translates to %+.1fmm on SLR Boost (30mm clamp window [%.1f, %+.1f] mm; feasible: %t; 45mm clamp feasible: %t due to %.0fmm rail).",
					transSlrStd.TargetClampOffset, transSlrStd.TargetClampMin, transSlrStd.TargetClampMax, transSlrStd.IsFeasible, transSlrDropBest.IsFeasible, slr.RailLengthMm),
			},
		})
	}

	if err := db.renderFooters(title, subtitle, headers, rows, footers); err != nil {
		db.fail("Error: %v", err)
		return 1
	}

	type seatpostOption struct {
		name    string
		setback float64
		clamp   float64
	}
	posts := []seatpostOption{
		{"0mm Inline Post", 0.0, model.DefaultSeatpostClampLengthMm},
		{"Redshift ShockStop (static 7mm)", 7.0, model.DefaultSeatpostClampLengthMm},
		{"Redshift ShockStop (sagged 11mm)", 11.0, model.DefaultSeatpostClampLengthMm},
		{"Standard 15mm Setback", 15.0, model.DefaultSeatpostClampLengthMm},
		{"Standard 25mm Setback", 25.0, model.DefaultSeatpostClampLengthMm},
		{"Redshift + Drop Best (static 35mm)", 35.0, model.DropBestClampLengthMm},
		{"Redshift + Drop Best (sagged 39mm)", 39.0, model.DropBestClampLengthMm},
	}

	// 2. Consolidated Comparison Table across all catalog saddles (when no specific saddle argument is given)
	if len(posArgs) == 0 && hasSQ && hasAspin && hasSLR {

		compHeaders := []string{
			"Seatpost Hardware",
			"Hardware Setback",
			"SQlab 610 Ideal STA",
			"Aspin Ideal STA",
			"SLR Boost Ideal STA",
			"Safe Range (SQlab)",
			"Safe Range (Aspin)",
			"Safe Range (SLR Boost)",
		}
		var compRows [][]string
		maxSQAspinDiff := model.Deg(0.0)
		minSLRDelta := model.Deg(math.Inf(1))
		maxSLRDelta := model.Deg(math.Inf(-1))
		var aspin39, sq39 model.Deg

		for _, p := range posts {
			shSQ := sqlab.TranslateSaddleHeightFromRef(sqlab, saddleHeight, refBrpSetback)
			shAspin := aspin.TranslateSaddleHeightFromRef(sqlab, saddleHeight, refBrpSetback)
			shSLR := slr.TranslateSaddleHeightFromRef(sqlab, saddleHeight, refBrpSetback)

			resSQ := geometry.CalculateIdealSeatTubeAngle(sqlab, sitBoneSetback, shSQ, p.setback, p.name, p.clamp)
			resAspin := geometry.CalculateIdealSeatTubeAngle(aspin, sitBoneSetback, shAspin, p.setback, p.name, p.clamp)
			resSLR := geometry.CalculateIdealSeatTubeAngle(slr, sitBoneSetback, shSLR, p.setback, p.name, p.clamp)

			if d := (resSQ.IdealStaDeg - resAspin.IdealStaDeg).Abs(); d > maxSQAspinDiff {
				maxSQAspinDiff = d
			}
			slrDiff := resSLR.IdealStaDeg - resSQ.IdealStaDeg
			if slrDiff < minSLRDelta {
				minSLRDelta = slrDiff
			}
			if slrDiff > maxSLRDelta {
				maxSLRDelta = slrDiff
			}
			if p.setback == 39.0 {
				sq39 = resSQ.IdealStaDeg
				aspin39 = resAspin.IdealStaDeg
			}

			safeSLR := fmt.Sprintf("%.1f°–%.1f°", resSLR.MinStaDeg, resSLR.MaxStaDeg)
			if p.clamp > slr.RailLengthMm {
				safeSLR = "N/A (clamp > rail)"
			}

			compRows = append(compRows, []string{
				p.name,
				fmt.Sprintf("%.1f mm", p.setback),
				fmt.Sprintf("%.1f°", resSQ.IdealStaDeg),
				fmt.Sprintf("%.1f°", resAspin.IdealStaDeg),
				fmt.Sprintf("%.1f°", resSLR.IdealStaDeg),
				fmt.Sprintf("%.1f°–%.1f°", resSQ.MinStaDeg, resSQ.MaxStaDeg),
				fmt.Sprintf("%.1f°–%.1f°", resAspin.MinStaDeg, resAspin.MaxStaDeg),
				safeSLR,
			})
		}

		min39 := min(aspin39, sq39)
		max39 := max(aspin39, sq39)
		compTitle := "Ideal Seat Tube Angle"
		compSubtitle := fmt.Sprintf("Saddle Height: %.0fmm | Target Sit Bone Setback: %.0fmm behind BB", saddleHeight, sitBoneSetback)
		compFooters := []report.Footer{
			{
				Title: "Seat Tube Angle Interpretation & Comparison",
				Lines: []string{
					"At a given saddle height and seatpost setback, the Ideal STA directly reflects the saddle's rail-to-sit-bone geometry.",
					fmt.Sprintf("SQlab 610 (%.1fmm) and Berthoud Aspin (%.1fmm) track within %.2f° of each other across all listed seatposts.",
						sqlab.RailCenterToSitBonesMm, aspin.RailCenterToSitBonesMm, maxSQAspinDiff),
					fmt.Sprintf("Selle Italia SLR Boost (%.1fmm) places sit bones %.1fmm further rearward than SQlab 610, requiring a %.2f°–%.2f° steeper STA to achieve the same pelvic position.",
						slr.RailCenterToSitBonesMm, math.Abs(slr.RailCenterToSitBonesMm-sqlab.RailCenterToSitBonesMm), minSLRDelta, maxSLRDelta),
					fmt.Sprintf("With a 39mm sagged setup (Redshift + Drop Best), the Ideal STA is %.1f°–%.1f° for Aspin/SQlab at %.0fmm SH and %.0fmm sit-bone setback (note: 45mm clamps exceed the SLR Boost's %.0fmm straight rail).",
						min39, max39, saddleHeight, sitBoneSetback, slr.RailLengthMm),
				},
			},
		}

		if err := db.renderFooters(compTitle, compSubtitle, compHeaders, compRows, compFooters); err != nil {
			db.fail("Error: %v", err)
			return 1
		}

		// Emit Universal Compromise Seat Tube Angle table across all catalog saddles

		compromiseHeaders := []string{
			"Seatpost Hardware",
			"Hardware Setback",
			"Compromise STA",
			"Max Rail Offset",
			"Universal ±5mm Window",
			"Individual Saddle Offsets",
		}
		var compromiseRows [][]string

		for _, p := range posts {
			resComp := geometry.CalculateUniversalCompromiseSTA(saddles, sitBoneSetback, saddleHeight, p.setback, p.name, p.clamp)

			univWindowStr := "None"
			if resComp.HasUniversalMargin {
				univWindowStr = fmt.Sprintf("%.1f°–%.1f°", resComp.UniversalMarginMinSta, resComp.UniversalMarginMaxSta)
			}

			var offsetParts []string
			for _, so := range resComp.SaddleOffsets {
				label := so.SaddleID
				switch so.SaddleID {
				case "sqlab-610-ergolux":
					label = "SQlab 610"
				case "berthoud-aspin-open":
					label = "Aspin"
				case "selle-italia-slr-boost-superflow-l3":
					label = "SLR Boost"
				}
				offsetParts = append(offsetParts, fmt.Sprintf("%s: %+.1fmm", label, so.OffsetMm))
			}

			compromiseRows = append(compromiseRows, []string{
				p.name,
				fmt.Sprintf("%.1f mm", p.setback),
				fmt.Sprintf("%.1f°", resComp.CompromiseStaDeg),
				fmt.Sprintf("±%.1f mm", resComp.MaxRailOffsetMm),
				univWindowStr,
				strings.Join(offsetParts, ", "),
			})
		}

		compromiseTitle := "Universal Compromise Seat Tube Angle (All Saddles)"
		compromiseSubtitle := fmt.Sprintf("Saddle Height: %.0fmm | Target Sit Bone Setback: %.0fmm behind BB", saddleHeight, sitBoneSetback)
		compromiseFooters := []report.Footer{
			{
				Title: "Framebuilder Compromise Recommendations",
				Lines: []string{
					"A frame built with the Compromise STA guarantees that any saddle in the catalog mounts within the specified Max Rail Offset of dead-center.",
					"Between SQlab (-50.0mm sit bones), Aspin (-44.5mm), and SLR Boost (-65.3mm), the sit bone span across the catalog is 20.8mm, leading to a ±10.4mm compromise clamp offset.",
					"Due to the SLR Boost's rearward sit-bone placement and 40mm straight rails, universal ±5mm compatibility across all three saddles requires varying seatpost setback (e.g. 0–10mm setback for SLR Boost vs 15–25mm for SQlab/Aspin).",
				},
			},
		}

		if err := db.renderFooters(compromiseTitle, compromiseSubtitle, compromiseHeaders, compromiseRows, compromiseFooters); err != nil {
			db.fail("Error: %v", err)
			return 1
		}
	}

	// 3. Emit Ideal Seat Tube Angle detailed table for each saddle in the catalog
	var targetSaddles []model.SaddleSpec
	if len(posArgs) > 0 {
		targetSaddles = []model.SaddleSpec{selectedSaddle}
	} else {
		targetSaddles = saddles
	}

	for _, s := range targetSaddles {

		staHeaders := []string{
			"Seatpost Hardware",
			"Hardware Setback",
			"Seat Tube Setback",
			"Ideal STA",
			"Safe STA Range",
			"±5mm Rail Margin",
		}
		var staRows [][]string

		sh := saddleHeight
		if hasSQ {
			sh = s.TranslateSaddleHeightFromRef(sqlab, saddleHeight, refBrpSetback)
		}
		for _, p := range posts {
			res := geometry.CalculateIdealSeatTubeAngle(
				s,
				sitBoneSetback,
				sh,
				p.setback,
				p.name,
				p.clamp,
			)

			safeRange := fmt.Sprintf("%.1f°–%.1f°", res.MinStaDeg, res.MaxStaDeg)
			marginRange := fmt.Sprintf("%.1f°–%.1f°", res.IdealMarginMinStaDeg, res.IdealMarginMaxStaDeg)
			if p.clamp > s.RailLengthMm {
				safeRange = "N/A (clamp > rail)"
				marginRange = "N/A (clamp > rail)"
			}
			staRows = append(staRows, []string{
				p.name,
				fmt.Sprintf("%.1f mm", p.setback),
				fmt.Sprintf("%.1f mm", res.SeatTubeSetbackMm),
				fmt.Sprintf("%.1f°", res.IdealStaDeg),
				safeRange,
				marginRange,
			})
		}

		staTitle := fmt.Sprintf("Ideal Seat Tube Angle: %s", s.Name)
		staSubtitle := fmt.Sprintf("Equivalent Saddle Height: %.0fmm | Target Sit Bone Setback: %.0fmm behind BB (BRP Setback: %.0fmm)",
			sh, sitBoneSetback, s.BrpSetbackFromSitBones(sitBoneSetback))

		if err := db.render(staTitle, staSubtitle, staHeaders, staRows); err != nil {
			db.fail("Error: %v", err)
			return 1
		}
	}

	return 0
}

package cli

import (
	"flag"
	"fmt"
	"strings"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
)

func (a *App) runSaddles(args []string) int {
	fs := flag.NewFlagSet("saddles", flag.ContinueOnError)
	fs.SetOutput(a.Err)

	var saddleHeight float64
	var sitBoneSetback float64

	fs.Float64Var(&saddleHeight, "saddle-height", 746.0, "Rider radial saddle height from BB in mm (default: 746mm)")
	fs.Float64Var(&saddleHeight, "sh", 746.0, "Alias for --saddle-height")
	fs.Float64Var(&sitBoneSetback, "sit-bone-setback", 292.0, "Target sit bone setback behind BB in mm (default: 292mm)")
	fs.Float64Var(&sitBoneSetback, "sb", 292.0, "Alias for --sit-bone-setback")

	reordered := reorderFlagsForStdlib(fs, args, nil)
	if err := fs.Parse(reordered); err != nil {
		return 2
	}

	posArgs := fs.Args()
	var selectedSaddle model.SaddleSpec
	if len(posArgs) > 0 {
		query := strings.TrimSpace(posArgs[0])
		if s, ok := catalog.GetSaddle(query); ok {
			selectedSaddle = s
		} else {
			fmt.Fprintf(a.Err, "Error: unrecognized saddle %q\n", query)
			return 1
		}
	} else {
		selectedSaddle = catalog.DefaultSaddle()
	}

	saddles := catalog.AllSaddles()
	headers := []string{
		"Saddle ID",
		"Name",
		"Rail Length",
		"Extended Rear",
		"Rail -> BRP (70mm)",
		"Rail -> Sit Bones",
		"BRP -> Sit Bones",
	}
	var rows [][]string

	for _, s := range saddles {
		extStr := "0.0 mm"
		if s.ExtendedRearRailMm > 0 {
			extStr = fmt.Sprintf("+%.1f mm", s.ExtendedRearRailMm)
		}
		rows = append(rows, []string{
			s.ID,
			s.Name,
			fmt.Sprintf("%.0f mm", s.RailLengthMm),
			extStr,
			fmt.Sprintf("+%.1f mm", s.RailCenterToBrpMm),
			fmt.Sprintf("%.1f mm", s.RailCenterToSitBonesMm),
			fmt.Sprintf("%.1f mm", s.DeltaBrpToSitBonesMm()),
		})
	}

	title := fmt.Sprintf("Canonical Saddle Catalog (%d)", len(saddles))
	subtitle := "Origin (0.0mm) is the midpoint of the straight rail clamping zone; (+) forward toward nose, (-) rearward toward tail"

	var footers []FooterSection

	// 1. Cross-Saddle Translation Section (SQlab 610 vs Berthoud Aspin & Selle Italia SLR Boost)
	sqlab, hasSQ := catalog.GetSaddle("sqlab-610-ergolux")
	aspin, hasAspin := catalog.GetSaddle("berthoud-aspin-open")
	slr, hasSLR := catalog.GetSaddle("selle-italia-slr-boost-superflow-l3")
	if hasSQ && hasAspin && hasSLR {
		transAspinDropBest := geometry.TranslateSaddleFit(sqlab, -15.0, aspin, model.DropBestClampLengthMm)
		transAspinStd := geometry.TranslateSaddleFit(sqlab, 0.0, aspin, model.DefaultSeatpostClampLengthMm)
		transSlrDropBest := geometry.TranslateSaddleFit(sqlab, -15.0, slr, model.DropBestClampLengthMm)
		transSlrStd := geometry.TranslateSaddleFit(sqlab, 0.0, slr, model.DefaultSeatpostClampLengthMm)

		footers = append(footers, FooterSection{
			Title: "Cross-Saddle Fit Translation: SQlab 610 Ergolux vs. Catalog Saddles",
			Lines: []string{
				fmt.Sprintf("Pelvic Fit Equivalence (Aspin): Rail-to-sit-bone delta is only %.1f mm (Aspin -44.5mm vs SQlab -50.0mm), slackening STA by ~0.44° at %.0fmm SH.", transAspinDropBest.SitBoneDeltaMm, saddleHeight),
				fmt.Sprintf("Pelvic Fit Equivalence (SLR Boost): Sit bones sit 15.3 mm further rearward (SLR -65.3mm vs SQlab -50.0mm), steepening STA by ~1.20° at %.0fmm SH.", saddleHeight),
				"BRP Caliper Discrepancy: BRP lines vary widely due to nose taper (Aspin 88mm vs SQlab 73mm vs SLR Boost 83mm from sit bones), proving raw BRP misleads without resolving sit bones.",
				fmt.Sprintf("Clamp Translation to Aspin: Clamping SQlab at -15.0mm translates to %.1fmm on Aspin (window [%.1f, +%.1f] mm; feasible: %t); dead-center translates to +%.1fmm (window [%.1f, +%.1f] mm).",
					transAspinDropBest.TargetClampOffset, transAspinDropBest.TargetPhysicalClampMin, transAspinDropBest.TargetPhysicalClampMax, transAspinDropBest.IsPhysicallyFeasible,
					transAspinStd.TargetClampOffset, transAspinStd.TargetPhysicalClampMin, transAspinStd.TargetPhysicalClampMax),
				fmt.Sprintf("Clamp Translation to SLR Boost: Clamping SQlab at dead-center (0.0mm) translates to %+.1fmm on SLR Boost (30mm clamp window [%.1f, +%.1f] mm; feasible: %t; 45mm clamp feasible: %t due to 40mm rail).",
					transSlrStd.TargetClampOffset, transSlrStd.TargetPhysicalClampMin, transSlrStd.TargetPhysicalClampMax, transSlrStd.IsPhysicallyFeasible, transSlrDropBest.IsPhysicallyFeasible),
			},
		})
	}

	if err := RenderOutputWithFooter(a.Out, a.Format, title, subtitle, headers, rows, footers); err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
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
		if a.Format != FormatCSV {
			fmt.Fprintln(a.Out)
		}

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

		for _, p := range posts {
			resSQ := geometry.CalculateIdealSeatTubeAngle(sqlab, sitBoneSetback, saddleHeight, p.setback, p.name, p.clamp)
			resAspin := geometry.CalculateIdealSeatTubeAngle(aspin, sitBoneSetback, saddleHeight, p.setback, p.name, p.clamp)
			resSLR := geometry.CalculateIdealSeatTubeAngle(slr, sitBoneSetback, saddleHeight, p.setback, p.name, p.clamp)

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

		compTitle := "Ideal Seat Tube Angle"
		compSubtitle := fmt.Sprintf("Saddle Height: %.0fmm | Target Sit Bone Setback: %.0fmm behind BB", saddleHeight, sitBoneSetback)
		compFooters := []FooterSection{
			{
				Title: "Seat Tube Angle Interpretation & Comparison",
				Lines: []string{
					"At a given saddle height and seatpost setback, the Ideal STA directly reflects the saddle's rail-to-sit-bone geometry.",
					"SQlab 610 (-50.0mm) and Berthoud Aspin (-44.5mm) track within 0.45° of each other across all seatposts.",
					"Selle Italia SLR Boost (-65.3mm) places sit bones 15.3mm further rearward, requiring a ~1.2° steeper STA (or ~15mm less seatpost setback) to achieve the same pelvic position.",
					"With a 39mm sagged setup (Redshift + Drop Best), the Ideal STA is ~74.2°–74.5° for SQlab/Aspin (explaining why the Otso Fenrir L at 74.53° requires the Drop Best). Note: 45mm clamps exceed the SLR Boost's 40mm straight rail.",
				},
			},
		}

		if err := RenderOutputWithFooter(a.Out, a.Format, compTitle, compSubtitle, compHeaders, compRows, compFooters); err != nil {
			fmt.Fprintf(a.Err, "Error: %v\n", err)
			return 1
		}

		// Emit Universal Compromise Seat Tube Angle table across all catalog saddles
		if a.Format != FormatCSV {
			fmt.Fprintln(a.Out)
		}

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
		compromiseFooters := []FooterSection{
			{
				Title: "Framebuilder Compromise Recommendations",
				Lines: []string{
					"A frame built with the Compromise STA guarantees that any saddle in the catalog mounts within the specified Max Rail Offset of dead-center.",
					"Between SQlab (-50.0mm sit bones), Aspin (-44.5mm), and SLR Boost (-65.3mm), the sit bone span across the catalog is 20.8mm, leading to a ±10.4mm compromise clamp offset.",
					"Due to the SLR Boost's rearward sit-bone placement and 40mm straight rails, universal ±5mm compatibility across all three saddles requires varying seatpost setback (e.g. 0–10mm setback for SLR Boost vs 15–25mm for SQlab/Aspin).",
				},
			},
		}

		if err := RenderOutputWithFooter(a.Out, a.Format, compromiseTitle, compromiseSubtitle, compromiseHeaders, compromiseRows, compromiseFooters); err != nil {
			fmt.Fprintf(a.Err, "Error: %v\n", err)
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
		if a.Format != FormatCSV {
			fmt.Fprintln(a.Out)
		}

		staHeaders := []string{
			"Seatpost Hardware",
			"Hardware Setback",
			"Seat Tube Setback",
			"Ideal STA",
			"Safe STA Range",
			"±5mm Rail Margin",
		}
		var staRows [][]string

		for _, p := range posts {
			res := geometry.CalculateIdealSeatTubeAngle(
				s,
				sitBoneSetback,
				saddleHeight,
				p.setback,
				p.name,
				p.clamp,
			)

			staRows = append(staRows, []string{
				p.name,
				fmt.Sprintf("%.1f mm", p.setback),
				fmt.Sprintf("%.1f mm", res.SeatTubeSetbackMm),
				fmt.Sprintf("%.1f°", res.IdealStaDeg),
				fmt.Sprintf("%.1f°–%.1f°", res.MinStaDeg, res.MaxStaDeg),
				fmt.Sprintf("%.1f°–%.1f°", res.IdealMarginMinStaDeg, res.IdealMarginMaxStaDeg),
			})
		}

		staTitle := fmt.Sprintf("Ideal Seat Tube Angle: %s", s.Name)
		staSubtitle := fmt.Sprintf("Saddle Height: %.0fmm | Target Sit Bone Setback: %.0fmm behind BB (BRP Setback: %.0fmm)",
			saddleHeight, sitBoneSetback, s.BrpSetbackFromSitBones(sitBoneSetback))

		if err := RenderOutput(a.Out, a.Format, staTitle, staSubtitle, staHeaders, staRows); err != nil {
			fmt.Fprintf(a.Err, "Error: %v\n", err)
			return 1
		}
	}

	return 0
}

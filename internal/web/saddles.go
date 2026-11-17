package web

import (
	"fmt"
	"math"
	"net/url"
	"strings"

	"bikegeo/internal/catalog"
	"bikegeo/internal/geom2d"
	"bikegeo/internal/geometry"
	"bikegeo/internal/ideal"
	"bikegeo/internal/model"
	"bikegeo/internal/report"
)

const clampAssumption = "Clamp windows assume a symmetric footprint on the catalog rail span. On the SQlab, that span is the measured distance between rail marks. Verify usable rail length, rail shape and clamp clearance on the actual hardware."

type saddlePostExample struct {
	name            string
	perpendicularMm float64
	horizontalMm    float64
	clamp           float64
}

func (p saddlePostExample) description() string {
	if p.horizontalMm != 0 {
		return fmt.Sprintf("%.0f mm horizontal rearward", p.horizontalMm)
	}
	return fmt.Sprintf("%.0f mm perpendicular to seat axis", p.perpendicularMm)
}

// Without the horizontal offset, the same hardware geometry would put every
// saddle point that far forward. Solve for that translated point at the same Y.
func (p saddlePostExample) fitInputs(s model.SaddleSpec, sit geom2d.Point2) (setback, brpHeight float64) {
	sit.X += p.horizontalMm
	return -sit.X, s.BrpPointFromSitBones(sit).DistanceToOrigin()
}

func (p saddlePostExample) idealSTA(s model.SaddleSpec, sit geom2d.Point2) (geometry.IdealStaResult, error) {
	setback, height := p.fitInputs(s, sit)
	return geometry.CalculateIdealSeatTubeAngle(s, setback, height, p.perpendicularMm, p.name, p.clamp)
}

func modeledStaRange(result geometry.IdealStaResult) string {
	if !result.HasClampWindow {
		return "N/A (clamp > rail)"
	}
	return fmt.Sprintf("%.1f°–%.1f°", result.MinStaDeg, result.MaxStaDeg)
}

func (db *docb) saddles(v url.Values) int {
	fl := &floats{v: v}
	saddleHeight := fl.get("saddle-height", model.DefaultIdmatchSaddleHeightMm)
	sitBoneSetback := fl.get("sit-bone-setback", model.DefaultTargetSitBoneSetbackMm())
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
		"Catalog Rail Span",
		"Rail -> BRP (70mm)",
		"Rail Center to SIT",
		"BRP to SIT (rearward)",
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
	subtitle := "Offsets along the rails are positive toward the nose. SIT is the modeled support landmark."
	footers := []report.Footer{{Title: "Rail marks and clamp assumptions", Lines: []string{clampAssumption}}}
	sqlab := catalog.DefaultSaddle()
	aspin, hasAspin := catalog.GetSaddle("berthoud-aspin-open")
	slr, hasSLR := catalog.GetSaddle("selle-italia-slr-boost-superflow-l3")
	refSitBones := model.ReferenceSitBonePoint(saddleHeight, sitBoneSetback)
	if len(posArgs) == 0 {
		refBrpSetback := sqlab.BrpSetbackFromSitBones(sitBoneSetback)
		refSol, err := geometry.SolveSaddleSTAForBRPTarget(sqlab, saddleHeight, refBrpSetback, model.DefaultSeatpostSetbackMm, 0)
		if err != nil {
			db.fail("%v", err)
			return 1
		}
		refSTA := refSol.EffStaDeg()
		reference := geometry.BRPPointFromSaddlePlacement(refSol.Placement())
		lines := []string{fmt.Sprintf("Start with a centered %s at %.0f mm radial BRP height and %.1f mm SIT setback, using the comparison's zero-setback dropper and assumed %.0f mm clamp footprint.", sqlab.Name, saddleHeight, sitBoneSetback, model.DefaultSeatpostClampLengthMm)}
		for _, target := range saddles {
			if target.ID == sqlab.ID {
				continue
			}
			const clamp = model.DefaultSeatpostClampLengthMm
			trans, err := geometry.TranslateSaddleFit(reference, target, refSTA, clamp)
			if err != nil {
				db.fail("%v", err)
				return 1
			}
			lines = append(lines, fmt.Sprintf("%s has a %+.1f mm change in local rail-center-to-SIT offset. Preserving the modeled SIT point calls for %.1f mm BB-to-BRP height and a %+.1f mm clamp station (%s).",
				target.Name, trans.SitBoneDeltaMm, trans.TargetPoint.BrpPoint().DistanceToOrigin(), trans.TargetClampOffset, clampPositionStatus(target, clamp, trans.TargetClampOffset)))
		}
		footers = append(footers, report.Footer{Title: "Cross-Saddle Fit Translation", Lines: lines})
	}

	if err := db.renderFooters(title, subtitle, headers, rows, footers); err != nil {
		db.fail("Error: %v", err)
		return 1
	}

	// Manufacturer rearward offsets are treated as horizontal in these examples.
	// Keep them separate from the model's seat-perpendicular post input.
	const redshiftStatic, redshiftSagged, dropBestOffset = 7.0, 12.0, 27.0
	const standardClamp, adapterClamp = model.DefaultSeatpostClampLengthMm, model.DropBestClampLengthMm
	posts := []saddlePostExample{
		{name: "Zero-setback dropper (comparison)", clamp: standardClamp},
		{name: "Redshift ShockStop PRO (static example)", horizontalMm: redshiftStatic, clamp: standardClamp},
		{name: "Redshift ShockStop PRO (25% travel example)", horizontalMm: redshiftSagged, clamp: standardClamp},
		{name: "15 mm setback post", perpendicularMm: 15, clamp: standardClamp},
		{name: "25 mm setback post", perpendicularMm: 25, clamp: standardClamp},
		{name: "Redshift + Drop Best (static example)", horizontalMm: redshiftStatic + dropBestOffset, clamp: adapterClamp},
		{name: "Redshift + Drop Best (25% travel example)", horizontalMm: redshiftSagged + dropBestOffset, clamp: adapterClamp},
	}
	hardwareNote := fmt.Sprintf("The Redshift examples treat the nominal %.0f mm static and %.0f mm at 25%% travel as horizontal offsets. The rearward Drop Best example adds the nominal %.0f mm in the same direction and assumes a %.0f mm footprint. The fit report comes from the unsagged transfer. These examples use the same fit target.", redshiftStatic, redshiftSagged, dropBestOffset, adapterClamp)

	// 2. Consolidated Comparison Table across all catalog saddles (when no specific saddle argument is given)
	if len(posArgs) == 0 && hasAspin && hasSLR {

		compHeaders := []string{
			"Seatpost Hardware",
			"Assumed Offset",
			"SQlab 610 Ideal STA",
			"Aspin Ideal STA",
			"SLR Boost Ideal STA",
			"Modeled Range (SQlab)",
			"Modeled Range (Aspin)",
			"Modeled Range (SLR Boost)",
		}
		var compRows [][]string
		maxSQAspinDiff := model.Deg(0.0)
		minSLRDelta := model.Deg(math.Inf(1))
		maxSLRDelta := model.Deg(math.Inf(-1))

		for _, p := range posts {
			resSQ, err := p.idealSTA(sqlab, refSitBones)
			if err != nil {
				db.fail("Error: %v", err)
				return 1
			}
			resAspin, err := p.idealSTA(aspin, refSitBones)
			if err != nil {
				db.fail("Error: %v", err)
				return 1
			}
			resSLR, err := p.idealSTA(slr, refSitBones)
			if err != nil {
				db.fail("Error: %v", err)
				return 1
			}
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
			compRows = append(compRows, []string{
				p.name,
				p.description(),
				fmt.Sprintf("%.1f°", resSQ.IdealStaDeg),
				fmt.Sprintf("%.1f°", resAspin.IdealStaDeg),
				fmt.Sprintf("%.1f°", resSLR.IdealStaDeg),
				modeledStaRange(resSQ),
				modeledStaRange(resAspin),
				modeledStaRange(resSLR),
			})
		}

		compTitle := "Ideal Seat Tube Angle"
		compSubtitle := fmt.Sprintf("Reference Saddle Height: %.0fmm | Target SIT Setback: %.0fmm behind BB", saddleHeight, sitBoneSetback)
		compFooters := []report.Footer{
			{
				Title: "Seat Tube Angle Interpretation & Comparison",
				Lines: []string{
					"Ideal STA centers the clamp while placing the modeled SIT point at the target. The modeled ranges also use the assumed clamp footprint.",
					fmt.Sprintf("SQlab 610 (%.1fmm) and Berthoud Aspin (%.1fmm) track within %.2f° of each other across all listed seatposts.",
						sqlab.RailCenterToSitBonesMm, aspin.RailCenterToSitBonesMm, maxSQAspinDiff),
					fmt.Sprintf("Selle Italia SLR Boost (%.1fmm) places SIT %.1fmm further rearward of rail center than SQlab 610. Its centered angle differs by %+.2f° to %+.2f° across these examples.",
						slr.RailCenterToSitBonesMm, math.Abs(slr.RailCenterToSitBonesMm-sqlab.RailCenterToSitBonesMm), minSLRDelta, maxSLRDelta),
					hardwareNote,
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
			"Assumed Offset",
			"Compromise STA",
			"Max Rail Offset",
			"Shared STA Within ±5 mm",
			"Required Clamp Stations",
			"Within All Assumed Windows",
		}
		var compromiseRows [][]string

		for _, p := range posts {
			setback, height := p.fitInputs(sqlab, refSitBones)
			resComp, err := geometry.CalculateUniversalCompromiseSTA(saddles, setback, height, p.perpendicularMm, p.name, p.clamp)
			if err != nil {
				db.fail("Error: %v", err)
				return 1
			}

			univWindowStr := "None"
			if resComp.HasUniversalMargin {
				univWindowStr = fmt.Sprintf("%.1f°–%.1f°", resComp.UniversalMarginMinSta, resComp.UniversalMarginMaxSta)
			}

			var offsetParts []string
			var outside []string
			for i, so := range resComp.SaddleOffsets {
				offsetParts = append(offsetParts, fmt.Sprintf("%s: %+.1f mm", so.SaddleName, so.OffsetMm))
				if !saddles[i].ClampFitsAt(p.clamp, so.OffsetMm) {
					outside = append(outside, so.SaddleName)
				}
			}
			windowResult := "Yes"
			if len(outside) != 0 {
				windowResult = "Outside: " + strings.Join(outside, ", ")
			}

			compromiseRows = append(compromiseRows, []string{
				p.name,
				p.description(),
				fmt.Sprintf("%.1f°", resComp.CompromiseStaDeg),
				fmt.Sprintf("±%.1f mm", resComp.MaxRailOffsetMm),
				univWindowStr,
				strings.Join(offsetParts, ", "),
				windowResult,
			})
		}

		compromiseTitle := "Compromise Seat Tube Angle (All Saddles)"
		compromiseSubtitle := fmt.Sprintf("Reference Saddle Height: %.0fmm | Target SIT Setback: %.0fmm behind BB", saddleHeight, sitBoneSetback)
		compromiseFooters := []report.Footer{
			{
				Title: "How to read the compromise angle",
				Lines: []string{
					"The compromise angle minimizes the largest required offset from rail center. Some offsets may fall outside the modeled clamp window.",
					fmt.Sprintf("Rail-center support offsets: SQlab %+.1fmm, Aspin %+.1fmm, SLR Boost %+.1fmm. Modeled compromise rail offsets are listed per hardware option.", sqlab.RailCenterToSitBonesMm, aspin.RailCenterToSitBonesMm, slr.RailCenterToSitBonesMm),
					clampAssumption,
				},
			},
		}

		if err := db.renderFooters(compromiseTitle, compromiseSubtitle, compromiseHeaders, compromiseRows, compromiseFooters); err != nil {
			db.fail("Error: %v", err)
			return 1
		}
	}

	// 3. Emit Ideal Seat Tube Angle detailed table for each saddle in the catalog
	for _, s := range catalogSaddles {
		staHeaders := []string{
			"Seatpost Hardware",
			"Assumed Offset",
			"Seat Tube Setback",
			"Ideal STA",
			"Modeled STA Range",
			"±5mm Rail Margin",
		}
		var staRows [][]string

		sh := s.BrpPointFromSitBones(refSitBones).DistanceToOrigin()
		for _, p := range posts {
			res, err := p.idealSTA(s, refSitBones)
			if err != nil {
				db.fail("Error: %v", err)
				return 1
			}
			marginRange := fmt.Sprintf("%.1f°–%.1f°", res.IdealMarginMinStaDeg, res.IdealMarginMaxStaDeg)
			if !res.HasClampWindow {
				marginRange = "N/A (clamp > rail)"
			}
			staRows = append(staRows, []string{
				p.name,
				p.description(),
				fmt.Sprintf("%.1f mm", res.SeatTubeSetbackMm),
				fmt.Sprintf("%.1f°", res.IdealStaDeg),
				modeledStaRange(res),
				marginRange,
			})
		}

		staTitle := fmt.Sprintf("Ideal Seat Tube Angle: %s", s.Name)
		staSubtitle := fmt.Sprintf("Equivalent Saddle Height: %.1fmm | Target SIT Setback: %.0fmm behind BB (BRP Setback: %.0fmm)",
			sh, sitBoneSetback, s.BrpSetbackFromSitBones(sitBoneSetback))

		var notes []report.Footer
		if len(posArgs) != 0 {
			notes = []report.Footer{{Title: "Illustrative seatposts", Lines: []string{hardwareNote}}}
		}
		if err := db.renderFooters(staTitle, staSubtitle, staHeaders, staRows, notes); err != nil {
			db.fail("Error: %v", err)
			return 1
		}
		db.doc.Sections[len(db.doc.Sections)-1].Table.Columns[3].ID = string(ideal.MetricIdealSta)
	}

	return 0
}

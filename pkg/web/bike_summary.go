package web

import (
	"fmt"

	"bikegeo/pkg/geom2d"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/report"
)

func buildBikeSummaryTable(sc geometry.BikeScene) report.Table {
	solvedStr := "n/a (parallel)"
	if sol, ok := sc.SolveStemSpacerFit(sc.Cockpit.Stem.AngleDeg); ok {
		sign := "+"
		if sol.SpacerMm < 0 {
			sign = ""
		}
		solvedStr = fmt.Sprintf("%.1fmm %.0f° %s%.1fmm", sol.StemLengthMm, sol.StemAngleDeg, sign, sol.SpacerMm)
	}
	errVec := sc.TargetPointError()
	railStatus := "in window ✓"
	if sc.RailMarginMm < 0 {
		railStatus = fmt.Sprintf("%.1f mm short ✗", -sc.RailMarginMm)
	}
	ptStr := func(p geom2d.Point2) string { return fmt.Sprintf("(%.1f, %.1f) mm", p.X, p.Y) }

	rows := [][]string{
		{"Frame (active)", fmt.Sprintf("Reach %.1f mm, Stack %.1f mm, HTA %.1f°, STA %.1f°", sc.Reach(), sc.Stack(), sc.HtaDeg, sc.EffectiveStaDeg), fmt.Sprintf("Pitch Δθ %+.2f°, Trail %.1f mm, RC/WB %.1f%%", sc.PitchDeg, sc.TrailMm(), sc.RearCenterWheelbaseRatio()*100)},
		{"Stem / Spacer Fit (exact)", solvedStr, fmt.Sprintf("At current stem angle %.0f° to hit C*", sc.Cockpit.Stem.AngleDeg)},
		{"Target Error (C* − StemClamp)", fmt.Sprintf("(%.1f, %.1f) mm (|Δ|=%.1f mm)", errVec.X, errVec.Y, errVec.Norm()), fmt.Sprintf("TargetRef: %s", sc.TargetRef.Label())},
		{"RequiredStemClamp C*", ptStr(sc.RequiredStemClamp), "Stem clamp center required by TargetPoint + bar"},
		{"StemClamp", ptStr(sc.StemClamp), fmt.Sprintf("%.0fmm %.0f° stem + %.0fmm spacers", sc.Cockpit.Stem.LengthMm, sc.Cockpit.Stem.AngleDeg, sc.Cockpit.SpacerMm)},
		{"HandGrip", ptStr(sc.HandGrip), fmt.Sprintf("BRP–Grip %.1f mm, Grip–SitBones Y %+.1f mm", sc.GripDistFromBrpMm(), sc.GripRiseAboveSitBonesMm())},
		{"TargetPoint", ptStr(sc.TargetPoint), sc.TargetRef.Label()},
		{"SitBones", ptStr(sc.SitBones), fmt.Sprintf("Target X = %.1f mm", sc.Fit.TargetSitBonesXMm)},
		{"BRP", ptStr(sc.BRP), fmt.Sprintf("Required rail offset %+.1f mm, margin %.1f mm (%s)", sc.RequiredRailOffsetMm, sc.RailMarginMm, railStatus)},
	}
	t := report.FromStrings(
		"bike-scene",
		"Scene coordinates and solved hardware",
		fmt.Sprintf("TargetRef: %s | Saddle: %s | Handlebar: %s", sc.TargetRef.Label(), sc.Seat.Saddle.Name, sc.Cockpit.Handlebar.Name),
		[]string{"Quantity", "Value", "Notes"},
		rows,
	)
	annotateZones(&t)
	return t
}

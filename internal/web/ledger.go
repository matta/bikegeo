package web

import (
	"fmt"
	"math"
	"strings"

	"bikegeo/internal/catalog"
	"bikegeo/internal/geom2d"
	"bikegeo/internal/geometry"
	"bikegeo/internal/ideal"
	"bikegeo/internal/model"
	"bikegeo/internal/report"
	"bikegeo/internal/scoring"
)

func buildSceneForComparisonRow(row model.CanonicalComparisonRow, prof *ideal.Profile, referenceHeightMm float64) (geometry.BikeScene, error) {
	frameSize, err := geometry.FrameSizeFromBikeGeometry(row.Bike)
	if err != nil {
		return geometry.BikeScene{}, err
	}
	if frameSize.ChainstayMm <= 0 {
		frameSize.ChainstayMm = math.Max(430.0, math.Abs(frameSize.BbDropMm)+1.0)
	}
	build := geometry.ComparisonBuild(row.Bike, frameSize, model.Some(row.EffectiveA2c), row.EffectiveRake)
	stem := model.StemSpec{LengthMm: 70, AngleDeg: -6}
	spacer := 0.0
	if row.Canonical.SelectedHB != nil {
		stem = row.Canonical.SelectedHB.Stem
		spacer = row.Canonical.SelectedHB.SpacerMm
	}
	bar := row.Canonical.Handlebar
	if bar.ID == "" {
		bar = catalog.DefaultHandlebar()
	}
	saddle := row.Canonical.Saddle
	if saddle.ID == "" {
		saddle = catalog.DefaultSaddle()
	}
	cockpit := geometry.CockpitSpec{
		HeadsetTopStackMm: model.DefaultHeadsetTopStackMm,
		SpacerMm:          spacer,
		Stem:              stem,
		StemClampHeightMm: model.StemThicknessMm,
		Handlebar:         bar,
		GripInsetMm:       model.DefaultGripInsetMm,
	}
	seat := geometry.SeatSpec{
		PostSetbackMm: model.DefaultSeatpostSetbackMm,
		ClampLengthMm: model.DefaultSeatpostClampLengthMm,
		Saddle:        saddle,
	}
	if row.Canonical.SelectedBRP != nil {
		seat.RailOffsetMm = row.Canonical.SelectedBRP.RailOffsetMm
	}
	targetRef := model.TargetHandGrip
	if prof != nil && prof.TargetRef.IsValid() {
		targetRef = prof.TargetRef
	}
	if row.Canonical.CockpitFitResult != nil && row.Canonical.CockpitFitResult.TargetRef.IsValid() {
		targetRef = row.Canonical.CockpitFitResult.TargetRef
	}
	target := geometry.DefaultRiderTarget(targetRef)
	if row.Canonical.CockpitFitResult != nil {
		ov := row.Canonical.CockpitFitResult.TargetOval
		if ov.CenterX != 0 || ov.CenterY != 0 {
			target.Point = geom2d.Point2{X: ov.CenterX, Y: ov.CenterY}
		}
	}
	fit := geometry.IdmatchSaddleFit(referenceHeightMm, saddle)
	fit.Target = target
	if prof != nil {
		fit.ReferenceBar = prof.ReferenceBar
	}
	if cfr := row.Canonical.CockpitFitResult; cfr != nil {
		fit.ReferenceBar = cfr.ReferenceBar
	}
	if !fit.ReferenceBar.IsSet() {
		fit.ReferenceBar = model.Some(catalog.DefaultHandlebar())
	}
	return geometry.BuildScene(frameSize, build, cockpit, seat, fit)
}

// buildComputationLedger builds a 5-stage arithmetic derivation table from
// intermediate BikeScene quantities already in memory (plus optional scoring details).
func buildComputationLedger(id, title string, sc geometry.BikeScene, cfr *model.CockpitFitResult, score *scoring.Result) report.Table {
	pt := func(p geom2d.Point2) string {
		return fmt.Sprintf("(%.1f, %.1f) mm", p.X, p.Y)
	}
	wbPub := "—"
	if wb, ok := sc.Frame.WheelbaseMm.Get(); ok && wb > 0 {
		wbPub = fmt.Sprintf("%.1f", wb)
	}
	cadEffA2c := sc.Frame.CadFork.EffectiveA2c(sc.Frame.CadSagPct)
	impliedStructA2c := sc.ChartL - sc.Frame.HtlMm
	dL := sc.StructuralA2cMm - (cadEffA2c + sc.Frame.CadLowerCupMm)
	dRake := sc.Build.Fork.RakeMm - sc.Frame.CadFork.RakeMm
	dSteerer := sc.Cockpit.HeadsetTopStackMm + sc.Cockpit.SpacerMm + sc.Cockpit.StemClampHeightMm/2.0
	barOff := geometry.GetHandlebarOffset(sc.Cockpit.Handlebar, sc.Cockpit.GripInsetMm, sc.Cockpit.BarRollDeg)
	errVec := sc.TargetPointError()

	rows := [][]string{
		{
			"1. Chart & Closure",
			"Published Chart",
			"Reach, Stack, HTA, HTL, BB Drop, CS, WB",
			fmt.Sprintf("R %.1f, S %.1f, HTA %.2f°, HTL %.1f, BBD %.1f, CS %.1f, WB %s",
				sc.Frame.ReachMm, sc.Frame.StackMm, sc.Frame.HtaDeg, sc.Frame.HtlMm, sc.Frame.BbDropMm, sc.Frame.ChainstayMm, wbPub),
			fmt.Sprintf("Size %s", sc.Frame.Size),
		},
		{
			"1. Chart & Closure",
			"Chart Front-End L_chart",
			"(Stack − BbDrop + Rake_cad·cos(HTA₀)) / sin(HTA₀)",
			fmt.Sprintf("(%.1f − %.1f + %.1f·cos(%.2f°)) / sin(%.2f°)",
				sc.Frame.StackMm, sc.Frame.BbDropMm, sc.Frame.CadFork.RakeMm, sc.Frame.HtaDeg, sc.Frame.HtaDeg),
			fmt.Sprintf("%.2f mm", sc.ChartL),
		},
		{
			"1. Chart & Closure",
			"Implied Structural A2C",
			"L_chart − HTL vs CadA2C_eff + CadLowerCup",
			fmt.Sprintf("%.2f − %.1f vs %.1f + %.1f", impliedStructA2c+sc.Frame.HtlMm, sc.Frame.HtlMm, cadEffA2c, sc.Frame.CadLowerCupMm),
			fmt.Sprintf("%.2f mm (Δ = %+.2f mm)", impliedStructA2c, sc.ResidualA2cMm),
		},
	}

	if fcRes, hasFc := sc.ResidualFcMm.Get(); hasFc {
		rows = append(rows, []string{
			"1. Chart & Closure",
			"Front Center Residual",
			"|FA_chart − BB| − Published FC",
			fmt.Sprintf("%.2f − %.1f", sc.ChartFrontAxle.DistanceToOrigin(), sc.Frame.FrontCenterMm.OrZero()),
			fmt.Sprintf("%+.2f mm", fcRes),
		})
	}
	if wbRes, hasWb := sc.ResidualWbMm.Get(); hasWb {
		rows = append(rows, []string{
			"1. Chart & Closure",
			"Wheelbase Residual",
			"(FA_chart.X − RA₀.X) − Published WB",
			fmt.Sprintf("%.2f − %.1f", sc.Frame.WheelbaseMm.OrZero()+wbRes, sc.Frame.WheelbaseMm.OrZero()),
			fmt.Sprintf("%+.2f mm", wbRes),
		})
	}

	rows = append(rows,
		[]string{
			"2. Fork, Cup & Pitch",
			"Effective A2C & Cup",
			"A2C_static − Travel·(Sag/100) + PhysicalLowerCup",
			fmt.Sprintf("%.1f − %.0f·(%.0f/100) + %.1f", sc.Build.Fork.StaticA2cMm, sc.Build.Fork.TravelMm, sc.Build.SagPct, sc.Build.PhysicalLowerCupMm),
			fmt.Sprintf("%.1f mm fork / %.1f mm struct", sc.ForkEffA2cMm, sc.StructuralA2cMm),
		},
		[]string{
			"2. Fork, Cup & Pitch",
			"Exact Pitch Δθ",
			"-atan2(FA₀.Y − RA₀.Y, WB₀)",
			fmt.Sprintf("ΔL = %+.1f mm, ΔRake = %+.1f mm", dL, dRake),
			fmt.Sprintf("%+.2f°", sc.PitchDeg),
		},
		[]string{
			"2. Fork, Cup & Pitch",
			"Leveled Frame Attitude",
			"HTA = HTA₀ − Δθ; leveled BB-relative frame",
			fmt.Sprintf("%.2f° − (%+.2f°)", sc.Frame.HtaDeg, sc.PitchDeg),
			fmt.Sprintf("HTA %.2f°, Reach %.1f, Stack %.1f, BBD %.1f, Trail %.1f mm", sc.HtaDeg, sc.Reach(), sc.Stack(), sc.BbDrop(), sc.TrailMm()),
		},
		[]string{
			"2. Fork, Cup & Pitch",
			"FC, WB & RC/WB",
			"FC = |FA − BB|, WB = FA.X − RA.X, RC/WB",
			fmt.Sprintf("FA.X %.1f − RA.X (%.1f)", sc.FrontAxle.X, sc.RearAxle.X),
			fmt.Sprintf("FC %.1f, WB %.1f mm (RC/WB %.1f%%)", sc.FrontCenter(), sc.Wheelbase(), sc.RearCenterWheelbaseRatio()*100),
		},
		[]string{
			"3. Rear Cockpit",
			"Effective STA & Clamp",
			"Solve |BRP − BB| = H_brp with post setback perpendicular to the seat axis",
			fmt.Sprintf("Nominal %.2f°, H_brp %.1f mm, PostSB %.1f mm", sc.NominalStaDeg, sc.Fit.BrpHeightMm, sc.Seat.PostSetbackMm),
			fmt.Sprintf("STA %.2f°, Clamp %s", sc.EffectiveStaDeg, pt(sc.SeatpostClamp)),
		},
		[]string{
			"3. Rear Cockpit",
			"Rail Offset & Margin",
			"RailOff_req for SitBones.X; Margin = HalfWin − |RailOff|",
			fmt.Sprintf("Target SB.X %.1f mm, HalfWin %.1f mm", sc.Fit.TargetSitBonesXMm, sc.Seat.HalfRailWindowMm()),
			fmt.Sprintf("%+.1f mm (margin %+.1f mm)", sc.RequiredRailOffsetMm, sc.RailMarginMm),
		},
		[]string{
			"3. Rear Cockpit",
			"SitBones & Workshop BRP",
			"BRP and SIT from rail center using the saddle’s rail angle",
			fmt.Sprintf("%s, rail angle %+.1f°, selected clamp %+.1f mm", sc.Seat.Saddle.Name, sc.Seat.Saddle.RailPitchDeg, sc.Seat.RailOffsetMm),
			fmt.Sprintf("SB %s, BRP %s", pt(sc.SitBones), pt(sc.BRP)),
		},
		[]string{
			"3. Rear Cockpit",
			"BB-to-BRP Saddle Height",
			"|BRP − BB|",
			pt(sc.BRP),
			fmt.Sprintf("%.1f mm", sc.BRP.DistanceTo(sc.BB)),
		},
		[]string{
			"3. Rear Cockpit",
			"Seatpost Extension Estimate",
			"Solved axial clamp distance − published seat tube length",
			func() string {
				if stl, ok := sc.Frame.SeatTubeLengthMm.Get(); ok {
					return fmt.Sprintf("%.1f − %.1f mm", sc.SaddlePlacement.SeatAxisDistanceMm, stl)
				}
				return "Seat tube length unavailable"
			}(),
			func() string {
				if ext, ok := sc.SeatpostExtensionMm(); ok {
					return fmt.Sprintf("%.1f mm", ext)
				}
				return "—"
			}(),
		},
		[]string{
			"4. Front Cockpit",
			"Steerer Clamp",
			"HT + (HeadsetTop + Spacer + StemH/2)·u_steer",
			fmt.Sprintf("(%.1f, %.1f) + (%.1f + %.1f + %.1f)·u(%.2f°)",
				sc.HeadTubeTop.X, sc.HeadTubeTop.Y, sc.Cockpit.HeadsetTopStackMm, sc.Cockpit.SpacerMm, sc.Cockpit.StemClampHeightMm/2.0, sc.HtaDeg),
			fmt.Sprintf("%s (d=%.1f mm)", pt(sc.SteererClamp), dSteerer),
		},
		[]string{
			"4. Front Cockpit",
			"Stem Clamp",
			"SteererClamp + StemLen·u_stem(HTA, StemAng)",
			fmt.Sprintf("(%.1f, %.1f) + %.1f mm @ %+.1f°", sc.SteererClamp.X, sc.SteererClamp.Y, sc.Cockpit.Stem.LengthMm, sc.Cockpit.Stem.AngleDeg),
			pt(sc.StemClamp),
		},
		[]string{
			"4. Front Cockpit",
			"Bar Offset & HandGrip",
			"StemClamp + BarOffset(ΔX, ΔY @ Roll)",
			fmt.Sprintf("%s, Δ=(%+.1f, %+.1f) @ %+.1f°", sc.Cockpit.Handlebar.Name, barOff.X, barOff.Y, sc.Cockpit.BarRollDeg),
			pt(sc.HandGrip),
		},
		[]string{
			"5. Fit Target & Solve",
			"Required Stem Clamp C*",
			fmt.Sprintf("TargetPoint (%s) → RequiredStemClamp", sc.Fit.Target.Ref.Label()),
			fmt.Sprintf("Target %s, Δ=(%+.1f, %+.1f)", pt(sc.Fit.Target.Point), errVec.X, errVec.Y),
			fmt.Sprintf("C* %s (|Δ|=%.1f mm)", pt(sc.RequiredStemClamp), errVec.Norm()),
		},
	)

	solvedVal := "n/a (parallel)"
	if sol, ok := sc.SolveStemSpacerFit(sc.Cockpit.Stem.AngleDeg); ok {
		sign := "+"
		if sol.SpacerMm < 0 {
			sign = ""
		}
		solvedVal = fmt.Sprintf("L = %.1f mm, Spacer = %s%.1f mm", sol.StemLengthMm, sign, sol.SpacerMm)
	}
	rows = append(rows, []string{
		"5. Fit Target & Solve",
		"2×2 Linear Fit Solve",
		"t·u_steer + L·u_stem = C* − HT",
		fmt.Sprintf("StemAng %+.1f°, HT (%.1f, %.1f)", sc.Cockpit.Stem.AngleDeg, sc.HeadTubeTop.X, sc.HeadTubeTop.Y),
		solvedVal,
	})

	lossParts := func(ids ...ideal.MetricID) ([]string, float64) {
		var parts []string
		total := 0.0
		if score != nil {
			for _, id := range ids {
				for _, cr := range score.Criteria {
					if cr.ID == id && !cr.IsDisplayOnly && !cr.Missing {
						parts = append(parts, fmt.Sprintf("%s %.1f", cr.Label, cr.PointsLost))
						total += cr.PointsLost
					}
				}
			}
		}
		return parts, total
	}
	if parts, total := lossParts(ideal.MetricBrpSetback, ideal.MetricRailOffset, ideal.MetricSeatpostExtension); len(parts) > 0 {
		rows = append(rows, []string{
			"5. Fit Target & Solve", "Stock Rail Selection",
			"Lowest points lost across stock rail positions",
			strings.Join(parts, " + "), fmt.Sprintf("%.1f points lost", total),
		})
	}
	if cfr != nil && cfr.SelectedHB.Stem.LengthMm > 0 {
		eval := geometry.EvaluatedPointForHB(cfr.SelectedHB, cfr.TargetRef, cfr.ReferenceBar)
		parts, total := lossParts(ideal.MetricHandlebarClampFit, ideal.MetricStemLength, ideal.MetricStemAngle, ideal.MetricSpacers)
		lostStr := fmt.Sprintf("R=%.2f", cfr.BestR)
		if len(parts) > 0 {
			lostStr = fmt.Sprintf("R=%.2f; lost %s = %.1f", cfr.BestR, strings.Join(parts, " + "), total)
		}
		rows = append(rows, []string{
			"5. Fit Target & Solve",
			"Cheapest Stock Build",
			"min over stem length × angle × spacers of Oval Fits + Stem Length + Stem Angle + Spacers points lost",
			fmt.Sprintf("Stem %.0fmm %+.0f° + %.0fmm sp (|Δ|=%.1f)",
				cfr.SelectedHB.Stem.LengthMm, cfr.SelectedHB.Stem.AngleDeg, cfr.SelectedHB.SpacerMm, math.Hypot(eval.X-cfr.TargetOval.CenterX, eval.Y-cfr.TargetOval.CenterY)),
			lostStr,
		})
	}

	return report.FromStrings(
		id,
		title,
		fmt.Sprintf("TargetRef: %s | Saddle: %s | Handlebar: %s", sc.Fit.Target.Ref.Label(), sc.Seat.Saddle.Name, sc.Cockpit.Handlebar.Name),
		[]string{"Stage", "Quantity", "Plain Formula", "Substituted Inputs", "Result"},
		rows,
	)
}

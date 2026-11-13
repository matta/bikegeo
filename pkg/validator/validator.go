package validator

import (
	"fmt"
	"math"
	"strings"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
)

// ValidateBikeGeometry validates a bike dataset for internal consistency,
// hardtail CAD conventions (static vs. sagged charts), headset cup requirements,
// and mathematical coherence between Stack, Reach, A2C, HTL, and BB Drop.
func ValidateBikeGeometry(bike model.BikeGeometry) model.ValidationReport {
	var issues []model.ValidationIssue

	// 1. Essential Missing Variables Check
	type reqField struct {
		name  string
		label string
		val   float64
	}
	required := []reqField{
		{name: "reach", label: "Reach", val: bike.Reach},
		{name: "stack", label: "Stack", val: bike.Stack},
		{name: "hta_deg", label: "Head Tube Angle (HTA)", val: float64(bike.HtaDeg)},
		{name: "sta_deg", label: "Seat Tube Angle (STA)", val: float64(bike.StaDeg)},
		{name: "htl", label: "Head Tube Length (HTL)", val: bike.Htl},
		{name: "bb_drop", label: "Bottom Bracket Drop", val: bike.BbDrop},
		{name: "fork_rake", label: "Fork Rake / Offset", val: bike.ForkRake},
		{name: "wheelbase", label: "Wheelbase", val: bike.Wheelbase},
	}

	for _, req := range required {
		if req.val <= 0 {
			issues = append(issues, model.ValidationIssue{
				ID:       fmt.Sprintf("missing_%s", req.name),
				Field:    req.name,
				Severity: model.SeverityError,
				Title:    fmt.Sprintf("Missing Essential Variable: %s", req.label),
				Message:  fmt.Sprintf("%s is required to execute the mathematical proofs.", req.label),
				Details:  "The Master Stack Equation and dynamic frame pitching require non-zero physical values.",
			})
		}
	}

	if bike.GetCadA2c() <= 0 {
		issues = append(issues, model.ValidationIssue{
			ID:       "missing_fork_a2c",
			Field:    "cad_fork.a2c_mm",
			Severity: model.SeverityError,
			Title:    "Missing Essential Variable: Fork Axle-to-Crown (A2C)",
			Message:  "CAD Fork A2C is required to verify the Master Stack Equation.",
		})
	}

	if bike.HeadsetCupType == "" {
		issues = append(issues, model.ValidationIssue{
			ID:       "missing_headset_cup_type",
			Field:    "headset_cup_type",
			Severity: model.SeverityError,
			Title:    "Missing Essential Variable: Headset Cup Type",
			Message:  "headset_cup_type (EC34, EC44, ZS44, ZS56, IS42, or IS52) is required.",
		})
	} else if !bike.HeadsetCupType.IsValid() {
		issues = append(issues, model.ValidationIssue{
			ID:       "invalid_headset_cup_type",
			Field:    "headset_cup_type",
			Severity: model.SeverityError,
			Title:    "Unrecognized Headset Cup Type",
			Message:  fmt.Sprintf("Headset cup type %q is not a recognized SHIS standard (EC34, EC44, ZS44, ZS56, IS42, IS52).", bike.HeadsetCupType),
			Expected: "EC34, EC44, ZS44, ZS56, IS42, or IS52",
			Actual:   string(bike.HeadsetCupType),
		})
	}

	if bike.GeoHeadsetSpacerMm < 0 || bike.GeoHeadsetSpacerMm > 25.0 {
		issues = append(issues, model.ValidationIssue{
			ID:       "invalid_geo_headset_spacer_mm",
			Field:    "geo_headset_spacer_mm",
			Severity: model.SeverityError,
			Title:    "Invalid CAD Headset Spacer (mm)",
			Message:  fmt.Sprintf("geo_headset_spacer_mm of %.1fmm is outside valid range (0mm - 25mm).", bike.GeoHeadsetSpacerMm),
			Expected: "0.0 - 25.0 mm",
			Actual:   fmt.Sprintf("%.1f mm", bike.GeoHeadsetSpacerMm),
		})
	}

	if bike.GeoSlopMm != nil {
		issues = append(issues, validateGeoSlop(bike.GeoSlopMm)...)
	}

	if bike.ActualStaDeg != 0 {
		if bike.ActualStaDeg < 60.0 || bike.ActualStaDeg > 85.0 {
			issues = append(issues, model.ValidationIssue{
				ID:       "invalid_actual_sta_deg",
				Field:    "actual_sta_deg",
				Severity: model.SeverityError,
				Title:    "Invalid Actual Seat Tube Angle",
				Message:  fmt.Sprintf("actual_sta_deg of %.1f° is outside plausible range (60.0° - 85.0°).", bike.ActualStaDeg),
				Expected: "60.0 - 85.0 deg",
				Actual:   fmt.Sprintf("%.1f deg", bike.ActualStaDeg),
			})
		}
	}

	if bike.SeatTubeOffsetXMm != nil {
		if *bike.SeatTubeOffsetXMm < 0 || *bike.SeatTubeOffsetXMm > 100.0 {
			issues = append(issues, model.ValidationIssue{
				ID:       "invalid_seat_tube_offset_x_mm",
				Field:    "seat_tube_offset_x_mm",
				Severity: model.SeverityError,
				Title:    "Invalid Seat Tube Forward Offset",
				Message:  fmt.Sprintf("seat_tube_offset_x_mm of %.1fmm is outside plausible range (0.0mm - 100.0mm).", *bike.SeatTubeOffsetXMm),
				Expected: "0.0 - 100.0 mm",
				Actual:   fmt.Sprintf("%.1f mm", *bike.SeatTubeOffsetXMm),
			})
		}
	}

	hasError := false
	for _, issue := range issues {
		if issue.Severity == model.SeverityError {
			hasError = true
			break
		}
	}

	effectiveCupStack := bike.HeadsetCupType.CupStackMm()
	geoSpacerMm := bike.GetGeoHeadsetSpacerMm()

	if hasError {
		return model.ValidationReport{
			BikeID:              bike.ID,
			IsValid:             false,
			Issues:              issues,
			GeometricA2c:        0,
			GeoHeadsetSpacerMm:  geoSpacerMm,
			CupStack:            effectiveCupStack,
			TotalStructuralA2c:  0,
			InferredPublishMode: model.ModeUnspecified,
		}
	}

	// 2. Physical Range Plausibility Checks
	if bike.HtaDeg < 55.0 || bike.HtaDeg > 76.0 {
		issues = append(issues, model.ValidationIssue{
			ID:       "hta_range",
			Field:    "hta_deg",
			Severity: model.SeverityError,
			Title:    "Invalid Head Tube Angle",
			Message:  fmt.Sprintf("HTA of %.1f° is outside valid range (55° - 76°).", bike.HtaDeg),
			Actual:   fmt.Sprintf("%.1f°", bike.HtaDeg),
		})
	}

	if bike.StaDeg < 65.0 || bike.StaDeg > 82.0 {
		issues = append(issues, model.ValidationIssue{
			ID:       "sta_range",
			Field:    "sta_deg",
			Severity: model.SeverityError,
			Title:    "Invalid Seat Tube Angle",
			Message:  fmt.Sprintf("STA of %.1f° is outside valid range (65° - 82°).", bike.StaDeg),
			Actual:   fmt.Sprintf("%.1f°", bike.StaDeg),
		})
	}

	if bike.BbDrop < 15.0 || bike.BbDrop > 95.0 {
		issues = append(issues, model.ValidationIssue{
			ID:       "bb_drop_range",
			Field:    "bb_drop",
			Severity: model.SeverityError,
			Title:    "Invalid Bottom Bracket Drop",
			Message:  fmt.Sprintf("BB drop of %.1fmm is outside valid mountain/gravel bike range (15mm - 95mm).", bike.BbDrop),
			Actual:   fmt.Sprintf("%.1f mm", bike.BbDrop),
		})
	}

	// Fork Travel & Static A2C Plausibility Check (for 29er suspension forks)
	forkTravel := bike.CadFork.ForkTravel()
	staticA2c := bike.CadFork.StaticA2c()
	if forkTravel > 0 && staticA2c > 0 {
		minStaticA2c := forkTravel + 400.0
		maxStaticA2c := forkTravel + 425.0
		if staticA2c < minStaticA2c || staticA2c > maxStaticA2c {
			issues = append(issues, model.ValidationIssue{
				ID:       "implausible_fork_a2c",
				Field:    "cad_fork.a2c_mm",
				Severity: model.SeverityError,
				Title:    "Implausible Suspension Fork Axle-to-Crown (A2C)",
				Message:  fmt.Sprintf("Static fork A2C of %.1fmm for %.0fmm travel is outside plausible 29er physical range [%.1fmm, %.1fmm].", staticA2c, forkTravel, minStaticA2c, maxStaticA2c),
				Expected: fmt.Sprintf("%.1f - %.1f mm", minStaticA2c, maxStaticA2c),
				Actual:   fmt.Sprintf("%.1f mm", staticA2c),
			})
		}
	}

	// 3. CAD Headset Spacer vs. Physical Headset Cup Type Plausibility Check
	minSpacer, maxSpacer := bike.HeadsetCupType.ExpectedSpacerRangeMm()
	if geoSpacerMm < minSpacer || geoSpacerMm > maxSpacer {
		issues = append(issues, model.ValidationIssue{
			ID:       "geo_spacer_cup_mismatch",
			Field:    "geo_headset_spacer_mm",
			Severity: model.SeverityWarning,
			Title:    "CAD Headset Spacer Differs From Physical Headset Cup",
			Message:  fmt.Sprintf("CAD geometry was drawn with a %.1fmm headset spacer, whereas %s physically uses %.0f–%.0fmm (standard %.0fmm).", geoSpacerMm, bike.HeadsetCupType, minSpacer, maxSpacer, effectiveCupStack),
			Details:  "Validation uses geo_headset_spacer_mm to verify the published chart, while comparisons pitch the frame using the physical headset_cup_type stack.",
			Expected: fmt.Sprintf("%.0f - %.0f mm (%s)", minSpacer, maxSpacer, bike.HeadsetCupType),
			Actual:   fmt.Sprintf("%.1f mm (CAD)", geoSpacerMm),
		})
	}

	// 4. Reverse-Engineering Geometric Axle-to-Crown (Master Stack Equation Check)
	structuralA2c, a2cErr := geometry.CalculateGeometricA2c(
		bike.Stack,
		bike.BbDrop,
		bike.Htl,
		bike.HtaDeg,
		bike.ForkRake,
	)
	if a2cErr != nil {
		issues = append(issues, model.ValidationIssue{
			ID:       "invalid_steerer_geometry",
			Field:    "hta_deg",
			Severity: model.SeverityError,
			Title:    "Invalid Steerer Geometry",
			Message:  a2cErr.Error(),
		})
	}
	calculatedForkA2c := structuralA2c - geoSpacerMm

	expectedForkA2c := bike.GetCadA2c()
	statedStaticA2c := staticA2c
	if statedStaticA2c <= 0 {
		statedStaticA2c = expectedForkA2c
	}

	inferredMode := model.ModeUnspecified
	var a2cDiff *float64
	var a2cBudgetPtr, fcBudgetPtr, wbBudgetPtr *model.ErrorBudget

	a2cBudget, hasA2cBudget := geometry.A2cBudget(bike)
	if expectedForkA2c > 0 && hasA2cBudget {
		a2cBudgetPtr = &a2cBudget
		diff := geometry.Round(a2cBudget.Residual, 1)
		a2cDiff = &diff
		budget := budgetSummary(a2cBudget)

		if a2cBudget.Within() {
			if bike.CadFork.SagPct > 0 {
				inferredMode = model.ModeSaggedCad
				issues = append(issues, model.ValidationIssue{
					ID:       "stack_sagged_confirmed",
					Severity: model.SeveritySuccess,
					Title:    "Stack & Sagged Fork A2C Match",
					Message:  fmt.Sprintf("Geometric A2C (%.1fmm) matches published %.0f%% sagged fork A2C (%.1fmm) within %.1fmm (%s, %.1fmm CAD spacer).", calculatedForkA2c, bike.CadFork.SagPct, expectedForkA2c, math.Abs(diff), budget, geoSpacerMm),
					Expected: fmt.Sprintf("%.1f mm", expectedForkA2c),
					Actual:   fmt.Sprintf("%.1f mm", calculatedForkA2c),
				})
			} else {
				inferredMode = model.ModeStaticCad
				issues = append(issues, model.ValidationIssue{
					ID:       "stack_static_confirmed",
					Severity: model.SeveritySuccess,
					Title:    "Stack & Fork A2C Match (Static CAD)",
					Message:  fmt.Sprintf("Geometric A2C (%.1fmm) matches stated fork A2C (%.1fmm) within %.1fmm (%s, %.1fmm CAD spacer).", calculatedForkA2c, expectedForkA2c, math.Abs(diff), budget, geoSpacerMm),
					Details:  "The manufacturer published geometry measured with an unweighted (0% sag) fork and the explicitly stated CAD headset spacer.",
					Expected: fmt.Sprintf("%.1f mm", expectedForkA2c),
					Actual:   fmt.Sprintf("%.1f mm", calculatedForkA2c),
				})
			}
			if notice, ok := budgetTierNotice("a2c", "stack", "Stack / Fork A2C", a2cBudget); ok {
				issues = append(issues, notice)
			}
		} else {
			inferredMode = model.ModeMismatchedCad
			issues = append(issues, model.ValidationIssue{
				ID:         "stack_a2c_mismatch",
				Field:      "stack",
				Severity:   model.SeverityError,
				Title:      "Geometric Inconsistency in Stack / A2C",
				Message:    fmt.Sprintf("The geometric A2C required to produce Stack %.1fmm (with %.1fmm CAD spacer) is %.1fmm, diverging from stated fork %.1fmm by %.1fmm (%s).", bike.Stack, geoSpacerMm, calculatedForkA2c, expectedForkA2c, diff, budget),
				Details:    fmt.Sprintf("Published Stack, HTL, BB Drop, HTA, CAD Headset Spacer, and Fork A2C do not satisfy the Master Stack Equation even with every published input at the edge of its rounding interval (largest contributors: %s).", topBudgetTerms(a2cBudget, 3)),
				Expected:   fmt.Sprintf("%.1f mm", expectedForkA2c),
				Actual:     fmt.Sprintf("%.1f mm", calculatedForkA2c),
				Difference: diff,
			})
		}
	}

	// 5. Front-Center & Wheelbase Consistency Check
	calculatedFcHoriz := geometry.CalculateFrontCenter(
		bike.Reach,
		bike.Htl,
		bike.HtaDeg,
		bike.ForkRake,
		structuralA2c,
	)
	calculatedFc3D := geometry.Round(
		geometry.CalculateFrontCenter3D(
			bike.Reach,
			bike.Htl,
			bike.HtaDeg,
			bike.ForkRake,
			structuralA2c,
			bike.BbDrop,
		),
		1,
	)

	// Horizontal front-center implied by published Front Center or Wheelbase, used to
	// emit a single sag-convention hint if either cross-check fails.
	var sagHintRefFcHoriz float64
	var sagHintSource string

	var fcDiff *float64
	if fcBudget, ok := geometry.FrontCenterBudget(bike); ok {
		fcBudgetPtr = &fcBudget
		diff := geometry.Round(math.Abs(fcBudget.Residual), 1)
		fcDiff = &diff
		budget := budgetSummary(fcBudget)
		if !fcBudget.Within() {
			issues = append(issues, model.ValidationIssue{
				ID:         "front_center_mismatch",
				Field:      "front_center",
				Severity:   model.SeverityError,
				Title:      "Front Center Inconsistency",
				Message:    fmt.Sprintf("Published Front Center (%.1fmm) differs from geometrically calculated 3D FC (%.1fmm) by %.1fmm (%s).", bike.FrontCenter, calculatedFc3D, diff, budget),
				Details:    fmt.Sprintf("Check if Reach, HTA, BB Drop, or Fork Rake is inconsistent in published geometry (largest rounding contributors: %s).", topBudgetTerms(fcBudget, 3)),
				Expected:   fmt.Sprintf("%.1f mm (calculated, 3D)", calculatedFc3D),
				Actual:     fmt.Sprintf("%.1f mm (published)", bike.FrontCenter),
				Difference: diff,
			})

			sagHintRefFcHoriz = bike.FrontCenter
			if bike.FrontCenter > bike.BbDrop {
				sagHintRefFcHoriz = math.Sqrt(bike.FrontCenter*bike.FrontCenter - bike.BbDrop*bike.BbDrop)
			}
			sagHintSource = "Front Center"
		} else {
			issues = append(issues, model.ValidationIssue{
				ID:       "front_center_ok",
				Severity: model.SeveritySuccess,
				Title:    "Front Center Geometrically Coherent",
				Message:  fmt.Sprintf("Published Front Center matches calculated 3D front-axle vector within %.1fmm (%s).", diff, budget),
			})
			if notice, ok := budgetTierNotice("front_center", "front_center", "Front Center", fcBudget); ok {
				issues = append(issues, notice)
			}
		}
	}

	if wbBudget, ok := geometry.WheelbaseBudget(bike); ok {
		wbBudgetPtr = &wbBudget
		csHorizontal := geometry.RearCenterHoriz(bike.ChainstayLength, bike.BbDrop)
		estimatedWb := geometry.Round(csHorizontal+calculatedFcHoriz, 1)
		wbDiff := geometry.Round(math.Abs(wbBudget.Residual), 1)
		budget := budgetSummary(wbBudget)

		if !wbBudget.Within() {
			issues = append(issues, model.ValidationIssue{
				ID:         "wheelbase_discrepancy",
				Field:      "wheelbase",
				Severity:   model.SeverityError,
				Title:      "Wheelbase & Chainstay Discrepancy",
				Message:    fmt.Sprintf("Wheelbase (%.1fmm) does not match Rear Center (%.1fmm) + Front Center Horiz (%.1fmm) = %.1fmm (Δ %.1fmm, %s).", bike.Wheelbase, csHorizontal, calculatedFcHoriz, estimatedWb, wbDiff, budget),
				Details:    fmt.Sprintf("Largest rounding contributors: %s.", topBudgetTerms(wbBudget, 3)),
				Expected:   fmt.Sprintf("%.1f mm (calculated)", estimatedWb),
				Actual:     fmt.Sprintf("%.1f mm (published)", bike.Wheelbase),
				Difference: wbDiff,
			})

			if sagHintSource == "" {
				sagHintRefFcHoriz = bike.Wheelbase - csHorizontal
				sagHintSource = "Wheelbase"
			}
		} else {
			issues = append(issues, model.ValidationIssue{
				ID:       "wheelbase_ok",
				Severity: model.SeveritySuccess,
				Title:    "Wheelbase Geometrically Coherent",
				Message:  fmt.Sprintf("Published Wheelbase matches horizontal Rear Center + Front Center within %.1fmm (%s).", wbDiff, budget),
			})
			if notice, ok := budgetTierNotice("wheelbase", "wheelbase", "Wheelbase", wbBudget); ok {
				issues = append(issues, notice)
			}
		}
	}

	if sagHintSource != "" {
		if hint, ok := stackReachSagHint(bike, structuralA2c, sagHintRefFcHoriz, sagHintSource); ok {
			issues = append(issues, hint)
		}
	}

	isValid := true
	for _, issue := range issues {
		if issue.Severity == model.SeverityError {
			isValid = false
			break
		}
	}

	return model.ValidationReport{
		BikeID:              bike.ID,
		IsValid:             isValid,
		Issues:              issues,
		GeometricA2c:        geometry.Round(calculatedForkA2c, 1),
		StatedA2c:           statedStaticA2c,
		ExpectedForkA2c:     expectedForkA2c,
		GeoHeadsetSpacerMm:  geoSpacerMm,
		CupStack:            effectiveCupStack,
		TotalStructuralA2c:  geometry.Round(structuralA2c, 1),
		A2cDifference:       a2cDiff,
		InferredPublishMode: inferredMode,
		FrontCenterCalc:     calculatedFc3D,
		FrontCenterDiff:     fcDiff,
		A2cBudget:           a2cBudgetPtr,
		FrontCenterBudget:   fcBudgetPtr,
		WheelbaseBudget:     wbBudgetPtr,
	}
}

func validateGeoSlop(slop *model.GeoSlopMm) []model.ValidationIssue {
	if slop == nil {
		return nil
	}
	var issues []model.ValidationIssue
	const minVal = 0.0
	check := func(field string, val *float64) {
		if val == nil {
			return
		}
		if *val < minVal || *val > 10.0 {
			issues = append(issues, model.ValidationIssue{
				ID:       "invalid_geo_slop_mm",
				Field:    "geo_slop_mm." + field,
				Severity: model.SeverityError,
				Title:    "Invalid Geometric Slop Tolerance",
				Message:  fmt.Sprintf("geo_slop_mm.%s of %.1fmm is outside valid range (%.1fmm - 10.0mm).", field, *val, minVal),
				Expected: fmt.Sprintf("%.1f - 10.0 mm", minVal),
				Actual:   fmt.Sprintf("%.1f mm", *val),
			})
		}
	}
	check("a2c", slop.A2c)
	check("front_center", slop.FrontCenter)
	check("wheelbase", slop.Wheelbase)
	return issues
}

// ValidateFrame is the centralized validation gate for an entire FrameModel.
// It validates frame metadata, all sizes across all configurations, and cross-configuration coherence.
// If ValidateFrame returns IsValid == false, the frame is in an error state and must not be used.
func ValidateFrame(frame model.FrameModel) model.FrameValidationReport {
	report := model.FrameValidationReport{
		FrameID:     frame.ID,
		IsValid:     true,
		SizeReports: make(map[string]model.ValidationReport),
	}

	if frame.Manufacturer == "" || frame.Model == "" {
		report.IsValid = false
		report.Issues = append(report.Issues, model.ValidationIssue{
			ID:       "missing_frame_metadata",
			Severity: model.SeverityError,
			Title:    "Missing Frame Metadata",
			Message:  "Frame manufacturer and model name are required.",
		})
	}

	if frame.HeadsetCupType == "" {
		report.IsValid = false
		report.Issues = append(report.Issues, model.ValidationIssue{
			ID:       "missing_headset_cup_type",
			Field:    "headset_cup_type",
			Severity: model.SeverityError,
			Title:    "Missing Headset Cup Type",
			Message:  "Frame must specify headset_cup_type (EC34, EC44, ZS44, ZS56, IS42, or IS52).",
		})
	} else if !frame.HeadsetCupType.IsValid() {
		report.IsValid = false
		report.Issues = append(report.Issues, model.ValidationIssue{
			ID:       "invalid_headset_cup_type",
			Field:    "headset_cup_type",
			Severity: model.SeverityError,
			Title:    "Unrecognized Headset Cup Type",
			Message:  fmt.Sprintf("Headset cup type %q is not a recognized SHIS standard (EC34, EC44, ZS44, ZS56, IS42, IS52).", frame.HeadsetCupType),
		})
	}

	if frame.GeoHeadsetSpacerMm < 0 || frame.GeoHeadsetSpacerMm > 25.0 {
		report.IsValid = false
		report.Issues = append(report.Issues, model.ValidationIssue{
			ID:       "invalid_geo_headset_spacer_mm",
			Field:    "geo_headset_spacer_mm",
			Severity: model.SeverityError,
			Title:    "Invalid CAD Headset Spacer (mm)",
			Message:  fmt.Sprintf("geo_headset_spacer_mm of %.1fmm is outside valid range (0mm - 25mm).", frame.GeoHeadsetSpacerMm),
		})
	}

	if frame.GeoSlopMm != nil {
		slopIssues := validateGeoSlop(frame.GeoSlopMm)
		if len(slopIssues) > 0 {
			report.IsValid = false
			report.Issues = append(report.Issues, slopIssues...)
		}
	}

	if frame.ActualStaDeg != 0 {
		if frame.ActualStaDeg < 60.0 || frame.ActualStaDeg > 85.0 {
			report.IsValid = false
			report.Issues = append(report.Issues, model.ValidationIssue{
				ID:       "invalid_actual_sta_deg",
				Field:    "actual_sta_deg",
				Severity: model.SeverityError,
				Title:    "Invalid Actual Seat Tube Angle",
				Message:  fmt.Sprintf("actual_sta_deg of %.1f° is outside plausible range (60.0° - 85.0°).", frame.ActualStaDeg),
				Expected: "60.0 - 85.0 deg",
				Actual:   fmt.Sprintf("%.1f deg", frame.ActualStaDeg),
			})
		}
	}

	if frame.SeatTubeOffsetXMm != nil {
		if *frame.SeatTubeOffsetXMm < 0 || *frame.SeatTubeOffsetXMm > 100.0 {
			report.IsValid = false
			report.Issues = append(report.Issues, model.ValidationIssue{
				ID:       "invalid_seat_tube_offset_x_mm",
				Field:    "seat_tube_offset_x_mm",
				Severity: model.SeverityError,
				Title:    "Invalid Seat Tube Forward Offset",
				Message:  fmt.Sprintf("seat_tube_offset_x_mm of %.1fmm is outside plausible range (0.0mm - 100.0mm).", *frame.SeatTubeOffsetXMm),
				Expected: "0.0 - 100.0 mm",
				Actual:   fmt.Sprintf("%.1f mm", *frame.SeatTubeOffsetXMm),
			})
		}
	}

	// Validate Cockpit & Stem/Spacer Envelope
	if frame.MaxSpacerMm < 0 || frame.MaxSpacerMm > 100.0 {
		report.IsValid = false
		report.Issues = append(report.Issues, model.ValidationIssue{
			ID:       "invalid_max_spacer_mm",
			Field:    "max_spacer_mm",
			Severity: model.SeverityError,
			Title:    "Invalid Maximum Spacer Stack",
			Message:  fmt.Sprintf("max_spacer_mm of %.1fmm is outside valid range (0mm - 100mm).", frame.MaxSpacerMm),
		})
	}

	if frame.CanonicalStem.LengthMm <= 0 {
		report.IsValid = false
		report.Issues = append(report.Issues, model.ValidationIssue{
			ID:       "invalid_canonical_stem",
			Field:    "canonical_stem",
			Severity: model.SeverityError,
			Title:    "Invalid Canonical Stem",
			Message:  fmt.Sprintf("canonical_stem length_mm must be positive, got %.1fmm.", frame.CanonicalStem.LengthMm),
		})
	}

	if len(frame.ExpectedStemRangeMm) > 0 {
		if len(frame.ExpectedStemRangeMm) != 2 || frame.ExpectedStemRangeMm[0] <= 0 || frame.ExpectedStemRangeMm[1] <= frame.ExpectedStemRangeMm[0] {
			report.IsValid = false
			report.Issues = append(report.Issues, model.ValidationIssue{
				ID:       "invalid_expected_stem_range",
				Field:    "expected_stem_range_mm",
				Severity: model.SeverityError,
				Title:    "Invalid Expected Stem Range",
				Message:  fmt.Sprintf("expected_stem_range_mm must contain exactly 2 positive values [min, max] where max > min, got %v.", frame.ExpectedStemRangeMm),
			})
		} else if frame.CanonicalStem.LengthMm > 0 {
			if frame.CanonicalStem.LengthMm < frame.ExpectedStemRangeMm[0] || frame.CanonicalStem.LengthMm > frame.ExpectedStemRangeMm[1] {
				report.IsValid = false
				report.Issues = append(report.Issues, model.ValidationIssue{
					ID:       "canonical_stem_outside_expected_range",
					Field:    "canonical_stem",
					Severity: model.SeverityError,
					Title:    "Canonical Stem Outside Expected Range",
					Message:  fmt.Sprintf("canonical_stem length (%.0fmm) is outside expected_stem_range_mm [%.0f, %.0f].", frame.CanonicalStem.LengthMm, frame.ExpectedStemRangeMm[0], frame.ExpectedStemRangeMm[1]),
				})
			}
		}
	}

	if len(frame.Stems) == 0 {
		report.IsValid = false
		report.Issues = append(report.Issues, model.ValidationIssue{
			ID:       "missing_stems",
			Field:    "stems",
			Severity: model.SeverityError,
			Title:    "Missing Stems",
			Message:  "Frame must explicitly define at least one stem option in stems.",
		})
	} else {
		var expandedStems []model.StemSpec
		for i, stemID := range frame.Stems {
			stemID = strings.TrimSpace(stemID)
			if stemID == "" {
				report.IsValid = false
				report.Issues = append(report.Issues, model.ValidationIssue{
					ID:       fmt.Sprintf("empty_stem_set_%d", i),
					Field:    fmt.Sprintf("stems[%d]", i),
					Severity: model.SeverityError,
					Title:    "Empty Stem Set ID",
					Message:  fmt.Sprintf("stems[%d] cannot be empty.", i),
				})
				continue
			}
			entry, found := catalog.GetCatalogStem(stemID)
			if !found {
				report.IsValid = false
				report.Issues = append(report.Issues, model.ValidationIssue{
					ID:       fmt.Sprintf("unknown_stem_set_%s", stemID),
					Field:    fmt.Sprintf("stems[%d]", i),
					Severity: model.SeverityError,
					Title:    "Unknown Stem Set",
					Message:  fmt.Sprintf("Stem set %q in stems[%d] is not defined in data/stems.json.", stemID, i),
				})
				continue
			}
			if len(entry.LengthRangeMm) != 2 || entry.LengthRangeMm[0] <= 0 || entry.LengthRangeMm[1] < entry.LengthRangeMm[0] {
				report.IsValid = false
				report.Issues = append(report.Issues, model.ValidationIssue{
					ID:       fmt.Sprintf("invalid_stem_set_range_%s", stemID),
					Field:    fmt.Sprintf("stems[%d]", i),
					Severity: model.SeverityError,
					Title:    "Invalid Stem Set Length Range",
					Message:  fmt.Sprintf("Stem set %q has invalid length range %v.", stemID, entry.LengthRangeMm),
				})
			}
			expandedStems = append(expandedStems, entry.Expand()...)
		}

		if frame.CanonicalStem.LengthMm > 0 {
			hasCanonical := false
			for _, s := range expandedStems {
				if math.Abs(s.LengthMm-frame.CanonicalStem.LengthMm) < 0.001 && (s.AngleDeg-frame.CanonicalStem.AngleDeg).Abs() < 0.001 {
					hasCanonical = true
					break
				}
			}
			if !hasCanonical {
				report.IsValid = false
				report.Issues = append(report.Issues, model.ValidationIssue{
					ID:       "canonical_stem_not_in_stems",
					Field:    "stems",
					Severity: model.SeverityError,
					Title:    "Canonical Stem Not in Stems List",
					Message:  fmt.Sprintf("canonical_stem (%.0fmm / %.1f°) must be included in the stems options list.", frame.CanonicalStem.LengthMm, frame.CanonicalStem.AngleDeg),
				})
			}
		}
	}

	// Validate Fork References
	cadForkIssues := ValidateForkRef(frame.ID, "cad_fork", frame.CadFork)
	if len(cadForkIssues) > 0 {
		report.IsValid = false
		report.Issues = append(report.Issues, cadForkIssues...)
	}

	if len(frame.Forks) == 0 {
		report.IsValid = false
		report.Issues = append(report.Issues, model.ValidationIssue{
			ID:       "missing_forks",
			Field:    "forks",
			Severity: model.SeverityError,
			Title:    "Missing Forks",
			Message:  "Frame must define at least one supported fork in forks.",
		})
	} else {
		for i, ref := range frame.Forks {
			fName := fmt.Sprintf("forks[%d]", i)
			rIssues := ValidateForkRef(frame.ID, fName, ref)
			if len(rIssues) > 0 {
				report.IsValid = false
				report.Issues = append(report.Issues, rIssues...)
			}
		}

		minA2c, maxA2c, hasRange := frame.EffectiveA2cRange()
		if hasRange {
			eff := frame.CadFork.A2cMm
			if eff > 0 && (eff < minA2c-0.5 || eff > maxA2c+0.5) {
				report.IsValid = false
				report.Issues = append(report.Issues, model.ValidationIssue{
					ID:       "cad_fork_outside_forks",
					Field:    "cad_fork",
					Severity: model.SeverityError,
					Title:    "CAD Fork Outside Supported Range",
					Message:  fmt.Sprintf("cad_fork effective A2C %.1fmm is outside allowed range [%.1fmm, %.1fmm].", eff, minA2c, maxA2c),
				})
			}
		}
	}

	sizes := frame.GetSizes()
	if len(sizes) == 0 || len(frame.Geometries) == 0 {
		report.IsValid = false
		report.Issues = append(report.Issues, model.ValidationIssue{
			ID:       "missing_geometries",
			Severity: model.SeverityError,
			Title:    "Missing Frame Geometries or Sizes",
			Message:  "Frame must define at least one size in geometries.",
		})
		return report
	}

	// Check adjustable chainstay boundaries
	if frame.HasAdjustableChainstay() {
		minCs := frame.GetMinChainstay()
		maxCs := frame.GetMaxChainstay()
		for sz, geo := range frame.Geometries {
			if geo.ChainstayLength > 0 {
				if geo.ChainstayLength < minCs || geo.ChainstayLength > maxCs {
					report.IsValid = false
					report.Issues = append(report.Issues, model.ValidationIssue{
						ID:       "chainstay_out_of_range",
						Severity: model.SeverityError,
						Title:    "Chainstay Outside Adjustable Range",
						Message:  fmt.Sprintf("Published chainstay %.1fmm for size %s is outside legal range [%.1fmm, %.1fmm].", geo.ChainstayLength, sz, minCs, maxCs),
					})
				}
			}
		}
	}

	for _, sz := range sizes {
		bike, err := geometry.MaterializeBikeGeometry(frame, sz)
		if err != nil {
			report.IsValid = false
			report.Issues = append(report.Issues, model.ValidationIssue{
				ID:       "materialize_error",
				Severity: model.SeverityError,
				Title:    fmt.Sprintf("Failed to Materialize Size %s", sz),
				Message:  err.Error(),
			})
			continue
		}
		valRep := ValidateBikeGeometry(bike)
		report.SizeReports[sz] = valRep
		if !valRep.IsValid {
			report.IsValid = false
		}
	}

	return report
}

// ValidateForkRef validates a ForkRef for required fields and invariant correctness against the catalog.
func ValidateForkRef(frameID string, fieldName string, fref model.ForkRef) []model.ValidationIssue {
	var issues []model.ValidationIssue

	if fref.A2cMm <= 0 {
		issues = append(issues, model.ValidationIssue{
			ID:       "missing_" + fieldName + "_a2c",
			Field:    fieldName + ".a2c_mm",
			Severity: model.SeverityError,
			Title:    fmt.Sprintf("Missing A2C in %s", fieldName),
			Message:  fmt.Sprintf("%s must specify positive a2c_mm.", fieldName),
		})
	}
	if fref.RakeMm <= 0 {
		issues = append(issues, model.ValidationIssue{
			ID:       "missing_" + fieldName + "_rake",
			Field:    fieldName + ".rake_mm",
			Severity: model.SeverityError,
			Title:    fmt.Sprintf("Missing Rake in %s", fieldName),
			Message:  fmt.Sprintf("%s must specify positive rake_mm.", fieldName),
		})
	}

	if fref.Fork == "" {
		suggestions := catalog.FindForkSuggestions(fref.A2cMm, fref.RakeMm)
		var msg string
		if len(suggestions) > 0 {
			var lines []string
			for _, s := range suggestions {
				if s.SagPct != nil {
					lines = append(lines, fmt.Sprintf("  - fork: %q, sag_pct: %.1f (static A2C: %.1fmm, rake: %.1fmm, %s)", s.ForkID, *s.SagPct, s.StaticA2c, s.Rake, s.ForkName))
				} else {
					lines = append(lines, fmt.Sprintf("  - fork: %q (rigid A2C: %.1fmm, rake: %.1fmm, %s)", s.ForkID, s.StaticA2c, s.Rake, s.ForkName))
				}
			}
			msg = fmt.Sprintf("%s is missing 'fork' catalog ID. Matching catalog options for A2C=%.1fmm, Rake=%.1fmm:\n%s", fieldName, fref.A2cMm, fref.RakeMm, strings.Join(lines, "\n"))
		} else {
			msg = fmt.Sprintf("%s is missing 'fork' catalog ID. No catalog forks match A2C=%.1fmm and Rake=%.1fmm.", fieldName, fref.A2cMm, fref.RakeMm)
		}
		issues = append(issues, model.ValidationIssue{
			ID:       "missing_" + fieldName + "_fork",
			Field:    fieldName + ".fork",
			Severity: model.SeverityError,
			Title:    fmt.Sprintf("Missing Fork ID in %s", fieldName),
			Message:  msg,
		})
		return issues
	}

	entry, found := catalog.GetCatalogFork(fref.Fork)
	if !found {
		issues = append(issues, model.ValidationIssue{
			ID:       "unknown_" + fieldName + "_fork",
			Field:    fieldName + ".fork",
			Severity: model.SeverityError,
			Title:    fmt.Sprintf("Unknown Fork ID in %s", fieldName),
			Message:  fmt.Sprintf("Fork %q in %s is not defined in data/forks.json.", fref.Fork, fieldName),
		})
		return issues
	}

	if entry.TravelMm == 0 {
		// Rigid fork
		if fref.SagPct != 0 {
			issues = append(issues, model.ValidationIssue{
				ID:       "invalid_" + fieldName + "_sag",
				Field:    fieldName + ".sag_pct",
				Severity: model.SeverityError,
				Title:    fmt.Sprintf("Invalid Sag on Rigid Fork in %s", fieldName),
				Message:  fmt.Sprintf("Rigid fork %q cannot have sag_pct (got %.1f%%).", fref.Fork, fref.SagPct),
			})
		}
		matched := false
		for _, opt := range entry.Options {
			if math.Abs(opt.A2cMm-fref.A2cMm) <= 0.5 && math.Abs(opt.RakeMm-fref.RakeMm) <= 0.5 {
				matched = true
				break
			}
		}
		if !matched {
			issues = append(issues, model.ValidationIssue{
				ID:       "invalid_" + fieldName + "_geometry",
				Field:    fieldName,
				Severity: model.SeverityError,
				Title:    fmt.Sprintf("Fork Geometry Mismatch in %s", fieldName),
				Message:  fmt.Sprintf("Rigid fork %q does not offer option with A2C=%.1fmm and Rake=%.1fmm.", fref.Fork, fref.A2cMm, fref.RakeMm),
			})
		}
	} else {
		// Suspension fork
		if fref.SagPct < 0 || fref.SagPct > 45 {
			issues = append(issues, model.ValidationIssue{
				ID:       "invalid_" + fieldName + "_sag_pct",
				Field:    fieldName + ".sag_pct",
				Severity: model.SeverityError,
				Title:    fmt.Sprintf("Invalid Sag Percentage in %s", fieldName),
				Message:  fmt.Sprintf("Sag percentage %.1f%% is outside plausible range [0%%, 45%%].", fref.SagPct),
			})
		}
		hasRake := false
		var allowedRakes []string
		for _, opt := range entry.Options {
			allowedRakes = append(allowedRakes, fmt.Sprintf("%.0fmm", opt.RakeMm))
			if math.Abs(opt.RakeMm-fref.RakeMm) <= 0.5 {
				hasRake = true
			}
		}
		if !hasRake {
			issues = append(issues, model.ValidationIssue{
				ID:       "unsupported_" + fieldName + "_rake",
				Field:    fieldName + ".rake_mm",
				Severity: model.SeverityError,
				Title:    fmt.Sprintf("Unsupported Rake in %s", fieldName),
				Message:  fmt.Sprintf("Fork %q does not support rake %.1fmm (available: %s).", fref.Fork, fref.RakeMm, strings.Join(allowedRakes, ", ")),
			})
			return issues
		}

		matched := false
		var candA2cs []string
		var candSags []string
		for _, opt := range entry.Options {
			if math.Abs(opt.RakeMm-fref.RakeMm) <= 0.5 {
				expectedA2c := opt.A2cMm - entry.TravelMm*(fref.SagPct/100.0)
				candA2cs = append(candA2cs, fmt.Sprintf("%.1fmm", expectedA2c))
				if math.Abs(expectedA2c-fref.A2cMm) <= 0.5 {
					matched = true
					break
				}
				diff := opt.A2cMm - fref.A2cMm
				if diff > 0.5 && diff <= entry.TravelMm*0.45 {
					cand := geometry.Round((diff/entry.TravelMm)*100.0, 1)
					candSags = append(candSags, fmt.Sprintf("sag_pct: %.1f%% (static A2C: %.1fmm)", cand, opt.A2cMm))
				}
			}
		}
		if !matched {
			if fref.SagPct == 0 && len(candSags) > 0 {
				msg := fmt.Sprintf("Suspension fork %q in %s requires 'sag_pct'. Matching sag for A2C=%.1fmm: %s", fref.Fork, fieldName, fref.A2cMm, strings.Join(candSags, ", "))
				issues = append(issues, model.ValidationIssue{
					ID:       "missing_" + fieldName + "_sag_pct",
					Field:    fieldName + ".sag_pct",
					Severity: model.SeverityError,
					Title:    fmt.Sprintf("Missing Sag Percentage in %s", fieldName),
					Message:  msg,
				})
			} else {
				issues = append(issues, model.ValidationIssue{
					ID:       "invalid_" + fieldName + "_invariant",
					Field:    fieldName,
					Severity: model.SeverityError,
					Title:    fmt.Sprintf("Fork Invariant Violated in %s", fieldName),
					Message:  fmt.Sprintf("Fork %q A2C=%.1fmm at %.1f%% sag does not match expected A2C (%s).", fref.Fork, fref.A2cMm, fref.SagPct, strings.Join(candA2cs, ", ")),
				})
			}
		}
	}

	return issues
}

// minSagHintA2cOffsetMm is the smallest equivalent A2C offset worth attributing to a sag
// convention mismatch. Typical design sag (15–30% of 100–160mm travel) moves A2C by ~15–50mm;
// smaller residuals are more likely rounding, rake, or chart typos.
const minSagHintA2cOffsetMm = 10.0

// stackReachSagHint diagnoses a failed Front Center / Wheelbase cross-check whose residual is
// large enough to be consistent with Stack and Reach having been published at different fork
// sag states. refFcHoriz is the horizontal front-center implied by the published source.
//
// The reported A2C offset is a first-order equivalent (HTA and BB drop held fixed), not a
// physical sag simulation: real sag also rotates the frame, steepening HTA and changing BB drop.
func stackReachSagHint(bike model.BikeGeometry, structuralA2c, refFcHoriz float64, source string) (model.ValidationIssue, bool) {
	if bike.Reach <= 0 || bike.HtaDeg <= 0 {
		return model.ValidationIssue{}, false
	}
	hta := bike.HtaDeg.Angle()
	cosHta := hta.Cos()
	sinHta := hta.Sin()
	if cosHta <= 0.05 {
		return model.ValidationIssue{}, false
	}

	expectedReach := refFcHoriz - (structuralA2c+bike.Htl)*cosHta - bike.ForkRake*sinHta
	reachOffset := bike.Reach - expectedReach
	equivA2cOffset := reachOffset / cosHta
	if math.Abs(equivA2cOffset) < minSagHintA2cOffsetMm {
		return model.ValidationIssue{}, false
	}

	return model.ValidationIssue{
		ID:       "stack_reach_sag_hint",
		Field:    "reach",
		Severity: model.SeverityWarning,
		Title:    "Possible Stack / Reach Sag Convention Mismatch",
		Message: fmt.Sprintf("Published Reach (%.1fmm) differs from the Reach implied by Stack and %s (%.1fmm) by %+.1fmm, equivalent to a %+.1fmm A2C offset at fixed HTA. This is consistent with Stack and Reach being published at different fork sag states.",
			bike.Reach, source, geometry.Round(expectedReach, 1), geometry.Round(reachOffset, 1), geometry.Round(-equivA2cOffset, 1)),
		Details:    "Other causes (wrong fork rake, chainstay convention, or a chart typo) can produce the same residual; confirm against the manufacturer's CAD before correcting Reach.",
		Expected:   fmt.Sprintf("%.1f mm (Reach consistent with Stack + %s)", geometry.Round(expectedReach, 1), source),
		Actual:     fmt.Sprintf("%.1f mm (published Reach)", bike.Reach),
		Difference: geometry.Round(math.Abs(reachOffset), 1),
	}, true
}

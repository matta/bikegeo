package validator

import (
	"fmt"
	"math"
	"strings"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
)

// CalculateDynamicWheelbaseTolerance computes the allowable wheelbase discrepancy based on
// propagation of uncertainty from input measurements (Reach ±0.5mm, Chainstay ±0.5mm,
// published Wheelbase ±0.5mm, and HTA precision step projection), with a standard minimum physical floor of 2.0mm.
func CalculateDynamicWheelbaseTolerance(bike model.BikeGeometry, structuralA2c float64) float64 {
	deltaReach := 0.5
	if bike.Reach > 0 && bike.Reach != math.Floor(bike.Reach) {
		deltaReach = 0.05
	}
	deltaCs := 0.5
	if bike.ChainstayLength > 0 && bike.ChainstayLength != math.Floor(bike.ChainstayLength) {
		deltaCs = 0.05
	}
	deltaWb := 0.5
	if bike.Wheelbase > 0 && bike.Wheelbase != math.Floor(bike.Wheelbase) {
		deltaWb = 0.05
	}

	deltaHtaDeg := 0.25
	if bike.HtaDeg == math.Floor(bike.HtaDeg) {
		deltaHtaDeg = 0.5
	} else if bike.HtaDeg*2 == math.Floor(bike.HtaDeg*2) {
		deltaHtaDeg = 0.25
	} else {
		deltaHtaDeg = 0.05
	}
	deltaHtaRad := deltaHtaDeg * math.Pi / 180.0

	frontEndLength := structuralA2c + bike.Htl
	htaRad := bike.HtaDeg * math.Pi / 180.0
	deltaFcHta := math.Abs(-frontEndLength*math.Sin(htaRad)+bike.ForkRake*math.Cos(htaRad)) * deltaHtaRad

	propagated := math.Sqrt(deltaReach*deltaReach + deltaCs*deltaCs + deltaWb*deltaWb + deltaFcHta*deltaFcHta)
	const floor = 2.0
	return math.Max(floor, geometry.Round(propagated, 1))
}

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
		{name: "hta_deg", label: "Head Tube Angle (HTA)", val: bike.HtaDeg},
		{name: "sta_deg", label: "Seat Tube Angle (STA)", val: bike.StaDeg},
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

	if bike.GeoHeadsetSpacerMm == nil {
		issues = append(issues, model.ValidationIssue{
			ID:       "missing_geo_headset_spacer_mm",
			Field:    "geo_headset_spacer_mm",
			Severity: model.SeverityError,
			Title:    "Missing Essential Variable: CAD Headset Spacer (mm)",
			Message:  "geo_headset_spacer_mm must be explicitly stated in the geometry JSON (e.g. 10 or 0).",
			Details:  "Every bike geometry must explicitly declare the below-headset spacer/cup height that the CAD chart was drawn with.",
		})
	} else if *bike.GeoHeadsetSpacerMm < 0 || *bike.GeoHeadsetSpacerMm > 25.0 {
		issues = append(issues, model.ValidationIssue{
			ID:       "invalid_geo_headset_spacer_mm",
			Field:    "geo_headset_spacer_mm",
			Severity: model.SeverityError,
			Title:    "Invalid CAD Headset Spacer (mm)",
			Message:  fmt.Sprintf("geo_headset_spacer_mm of %.1fmm is outside valid range (0mm - 25mm).", *bike.GeoHeadsetSpacerMm),
			Expected: "0.0 - 25.0 mm",
			Actual:   fmt.Sprintf("%.1f mm", *bike.GeoHeadsetSpacerMm),
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
	slop := bike.GetGeoSlopMm()

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
	structuralA2c, calculatedForkA2c := geometry.CalculateGeometricA2c(
		bike.Stack,
		bike.BbDrop,
		bike.Htl,
		bike.HtaDeg,
		bike.ForkRake,
		geoSpacerMm,
	)

	expectedForkA2c := bike.GetCadA2c()
	statedStaticA2c := staticA2c
	if statedStaticA2c <= 0 {
		statedStaticA2c = expectedForkA2c
	}

	inferredMode := model.ModeUnspecified
	var a2cDiff *float64
	a2cSlop := slop.GetA2c()

	if expectedForkA2c > 0 {
		diff := geometry.Round(expectedForkA2c-calculatedForkA2c, 1)
		a2cDiff = &diff

		if math.Abs(diff) <= a2cSlop {
			if bike.CadFork.SagPct != nil && *bike.CadFork.SagPct > 0 {
				inferredMode = model.ModeSaggedCad
				issues = append(issues, model.ValidationIssue{
					ID:       "stack_sagged_confirmed",
					Severity: model.SeveritySuccess,
					Title:    "Stack & Sagged Fork A2C Match",
					Message:  fmt.Sprintf("Geometric A2C (%.1fmm) matches published %.0f%% sagged fork A2C (%.1fmm) within %.1fmm (tolerance ±%.1fmm, %.1fmm CAD spacer).", calculatedForkA2c, *bike.CadFork.SagPct, expectedForkA2c, math.Abs(diff), a2cSlop, geoSpacerMm),
					Expected: fmt.Sprintf("%.1f mm", expectedForkA2c),
					Actual:   fmt.Sprintf("%.1f mm", calculatedForkA2c),
				})
			} else {
				inferredMode = model.ModeStaticCad
				issues = append(issues, model.ValidationIssue{
					ID:       "stack_static_confirmed",
					Severity: model.SeveritySuccess,
					Title:    "Stack & Fork A2C Match (Static CAD)",
					Message:  fmt.Sprintf("Geometric A2C (%.1fmm) matches stated fork A2C (%.1fmm) within %.1fmm (tolerance ±%.1fmm, %.1fmm CAD spacer).", calculatedForkA2c, expectedForkA2c, math.Abs(diff), a2cSlop, geoSpacerMm),
					Details:  "The manufacturer published geometry measured with an unweighted (0% sag) fork and the explicitly stated CAD headset spacer.",
					Expected: fmt.Sprintf("%.1f mm", expectedForkA2c),
					Actual:   fmt.Sprintf("%.1f mm", calculatedForkA2c),
				})
			}
		} else {
			inferredMode = model.ModeMismatchedCad
			issues = append(issues, model.ValidationIssue{
				ID:         "stack_a2c_mismatch",
				Field:      "stack",
				Severity:   model.SeverityError,
				Title:      "Geometric Inconsistency in Stack / A2C",
				Message:    fmt.Sprintf("The geometric A2C required to produce Stack %.1fmm (with %.1fmm CAD spacer) is %.1fmm, diverging from stated fork %.1fmm by %.1fmm (allowed slop ±%.1fmm).", bike.Stack, geoSpacerMm, calculatedForkA2c, expectedForkA2c, diff, a2cSlop),
				Details:    fmt.Sprintf("Published Stack, HTL, BB Drop, HTA, CAD Headset Spacer, and Fork A2C do not satisfy the Master Stack Equation within ±%.1fmm tolerance.", a2cSlop),
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

	var fcDiff *float64
	fcSlop := slop.GetFrontCenter()
	if bike.FrontCenter > 0 {
		diff := geometry.Round(math.Abs(bike.FrontCenter-calculatedFc3D), 1)
		fcDiff = &diff
		if diff > fcSlop {
			issues = append(issues, model.ValidationIssue{
				ID:         "front_center_mismatch",
				Field:      "front_center",
				Severity:   model.SeverityError,
				Title:      "Front Center Inconsistency",
				Message:    fmt.Sprintf("Published Front Center (%.1fmm) differs from geometrically calculated 3D FC (%.1fmm) by %.1fmm (allowed slop ±%.1fmm).", bike.FrontCenter, calculatedFc3D, diff, fcSlop),
				Details:    "Check if Reach, HTA, BB Drop, or Fork Rake is inconsistent in published geometry.",
				Expected:   fmt.Sprintf("%.1f mm (calculated)", calculatedFc3D),
				Actual:     fmt.Sprintf("%.1f mm (published)", bike.FrontCenter),
				Difference: diff,
			})
		} else {
			issues = append(issues, model.ValidationIssue{
				ID:       "front_center_ok",
				Severity: model.SeveritySuccess,
				Title:    "Front Center Geometrically Coherent",
				Message:  fmt.Sprintf("Published Front Center matches calculated point-to-point vector within %.1fmm (tolerance ±%.1fmm).", diff, fcSlop),
			})
		}
	}

	wbSlop := slop.GetWheelbase()
	if slop.Wheelbase == nil {
		wbSlop = CalculateDynamicWheelbaseTolerance(bike, structuralA2c)
	}
	if bike.Wheelbase > 0 && bike.ChainstayLength > 0 {
		csHorizontal := math.Sqrt(math.Max(0, bike.ChainstayLength*bike.ChainstayLength-bike.BbDrop*bike.BbDrop))
		estimatedWb := geometry.Round(csHorizontal+calculatedFcHoriz, 1)
		wbDiff := geometry.Round(math.Abs(bike.Wheelbase-estimatedWb), 1)

		if wbDiff > wbSlop {
			issues = append(issues, model.ValidationIssue{
				ID:         "wheelbase_discrepancy",
				Field:      "wheelbase",
				Severity:   model.SeverityError,
				Title:      "Wheelbase & Chainstay Discrepancy",
				Message:    fmt.Sprintf("Wheelbase (%.1fmm) does not match Rear Center (%.1fmm) + Front Center Horiz (%.1fmm) = %.1fmm (Δ %.1fmm, allowed slop ±%.1fmm).", bike.Wheelbase, csHorizontal, calculatedFcHoriz, estimatedWb, wbDiff, wbSlop),
				Expected:   fmt.Sprintf("%.1f mm (calculated)", estimatedWb),
				Actual:     fmt.Sprintf("%.1f mm (published)", bike.Wheelbase),
				Difference: wbDiff,
			})
		} else {
			issues = append(issues, model.ValidationIssue{
				ID:       "wheelbase_ok",
				Severity: model.SeveritySuccess,
				Title:    "Wheelbase Geometrically Coherent",
				Message:  fmt.Sprintf("Published Wheelbase matches horizontal Rear Center + Front Center within %.1fmm (tolerance ±%.1fmm).", wbDiff, wbSlop),
			})
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
	}
}

func validateGeoSlop(slop *model.GeoSlopMm) []model.ValidationIssue {
	if slop == nil {
		return nil
	}
	var issues []model.ValidationIssue
	check := func(field string, val *float64) {
		if val == nil {
			return
		}
		if *val < 0 || *val > 10.0 {
			issues = append(issues, model.ValidationIssue{
				ID:       "invalid_geo_slop_mm",
				Field:    "geo_slop_mm." + field,
				Severity: model.SeverityError,
				Title:    "Invalid Geometric Slop Tolerance",
				Message:  fmt.Sprintf("geo_slop_mm.%s of %.1fmm is outside valid range (0mm - 10mm).", field, *val),
				Expected: "0.0 - 10.0 mm",
				Actual:   fmt.Sprintf("%.1f mm", *val),
			})
		}
	}
	check("a2c", slop.A2c)
	check("front_center", slop.FrontCenter)
	check("wheelbase", slop.Wheelbase)
	check("cross_config", slop.CrossConfig)
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

	if frame.GeoHeadsetSpacerMm == nil {
		report.IsValid = false
		report.Issues = append(report.Issues, model.ValidationIssue{
			ID:       "missing_geo_headset_spacer_mm",
			Field:    "geo_headset_spacer_mm",
			Severity: model.SeverityError,
			Title:    "Missing CAD Headset Spacer (mm)",
			Message:  "Frame must explicitly specify geo_headset_spacer_mm (e.g. 10 or 0).",
		})
	} else if *frame.GeoHeadsetSpacerMm < 0 || *frame.GeoHeadsetSpacerMm > 25.0 {
		report.IsValid = false
		report.Issues = append(report.Issues, model.ValidationIssue{
			ID:       "invalid_geo_headset_spacer_mm",
			Field:    "geo_headset_spacer_mm",
			Severity: model.SeverityError,
			Title:    "Invalid CAD Headset Spacer (mm)",
			Message:  fmt.Sprintf("geo_headset_spacer_mm of %.1fmm is outside valid range (0mm - 25mm).", *frame.GeoHeadsetSpacerMm),
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
		if fref.SagPct != nil && *fref.SagPct != 0 {
			issues = append(issues, model.ValidationIssue{
				ID:       "invalid_" + fieldName + "_sag",
				Field:    fieldName + ".sag_pct",
				Severity: model.SeverityError,
				Title:    fmt.Sprintf("Invalid Sag on Rigid Fork in %s", fieldName),
				Message:  fmt.Sprintf("Rigid fork %q cannot have sag_pct (got %.1f%%).", fref.Fork, *fref.SagPct),
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
		if fref.SagPct == nil {
			if strings.HasPrefix(fieldName, "forks") {
				// Supported forks represent static fork travel limits (0% sag)
				matchedStatic := false
				for _, opt := range entry.Options {
					if math.Abs(opt.RakeMm-fref.RakeMm) <= 0.5 && math.Abs(opt.A2cMm-fref.A2cMm) <= 0.5 {
						matchedStatic = true
						break
					}
				}
				if matchedStatic {
					return issues
				}
			}
			var candSags []string
			for _, opt := range entry.Options {
				if math.Abs(opt.RakeMm-fref.RakeMm) <= 0.5 {
					diff := opt.A2cMm - fref.A2cMm
					if diff >= 0 && diff <= entry.TravelMm*0.45 {
						cand := geometry.Round((diff/entry.TravelMm)*100.0, 1)
						candSags = append(candSags, fmt.Sprintf("sag_pct: %.1f%% (static A2C: %.1fmm)", cand, opt.A2cMm))
					}
				}
			}
			msg := fmt.Sprintf("Suspension fork %q in %s requires 'sag_pct'.", fref.Fork, fieldName)
			if len(candSags) > 0 {
				msg += fmt.Sprintf(" Matching sag for A2C=%.1fmm: %s", fref.A2cMm, strings.Join(candSags, ", "))
			}
			issues = append(issues, model.ValidationIssue{
				ID:       "missing_" + fieldName + "_sag_pct",
				Field:    fieldName + ".sag_pct",
				Severity: model.SeverityError,
				Title:    fmt.Sprintf("Missing Sag Percentage in %s", fieldName),
				Message:  msg,
			})
		} else {
			if *fref.SagPct < 0 || *fref.SagPct > 45 {
				issues = append(issues, model.ValidationIssue{
					ID:       "invalid_" + fieldName + "_sag_pct",
					Field:    fieldName + ".sag_pct",
					Severity: model.SeverityError,
					Title:    fmt.Sprintf("Invalid Sag Percentage in %s", fieldName),
					Message:  fmt.Sprintf("Sag percentage %.1f%% is outside plausible range [0%%, 45%%].", *fref.SagPct),
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
			for _, opt := range entry.Options {
				if math.Abs(opt.RakeMm-fref.RakeMm) <= 0.5 {
					expectedA2c := opt.A2cMm - entry.TravelMm*(*fref.SagPct/100.0)
					candA2cs = append(candA2cs, fmt.Sprintf("%.1fmm", expectedA2c))
					if math.Abs(expectedA2c-fref.A2cMm) <= 0.5 {
						matched = true
						break
					}
				}
			}
			if !matched {
				issues = append(issues, model.ValidationIssue{
					ID:       "invalid_" + fieldName + "_invariant",
					Field:    fieldName,
					Severity: model.SeverityError,
					Title:    fmt.Sprintf("Fork Invariant Violated in %s", fieldName),
					Message:  fmt.Sprintf("Fork %q A2C=%.1fmm at %.1f%% sag does not match expected A2C (%s).", fref.Fork, fref.A2cMm, *fref.SagPct, strings.Join(candA2cs, ", ")),
				})
			}
		}
	}

	return issues
}

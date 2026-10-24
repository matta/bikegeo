package validator

import (
	"fmt"
	"math"

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

	if bike.ForkA2c <= 0 && bike.ForkTravelMm <= 0 {
		issues = append(issues, model.ValidationIssue{
			ID:       "missing_fork_a2c",
			Field:    "fork_a2c",
			Severity: model.SeverityError,
			Title:    "Missing Essential Variable: Fork Axle-to-Crown (A2C)",
			Message:  "Fork A2C (or travel_mm) is required to verify the Master Stack Equation.",
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

	// Fork Travel & Static A2C Plausibility Check (for 29er forks)
	if bike.ForkTravelMm > 0 && bike.ForkA2c > 0 {
		minStaticA2c := bike.ForkTravelMm + 400.0
		maxStaticA2c := bike.ForkTravelMm + 425.0
		if bike.ForkA2c < minStaticA2c || bike.ForkA2c > maxStaticA2c {
			issues = append(issues, model.ValidationIssue{
				ID:       "implausible_fork_a2c",
				Field:    "fork_a2c",
				Severity: model.SeverityError,
				Title:    "Implausible Suspension Fork Axle-to-Crown (A2C)",
				Message:  fmt.Sprintf("Static fork A2C of %.1fmm for %.0fmm travel is outside plausible 29er physical range [%.1fmm, %.1fmm].", bike.ForkA2c, bike.ForkTravelMm, minStaticA2c, maxStaticA2c),
				Expected: fmt.Sprintf("%.1f - %.1f mm", minStaticA2c, maxStaticA2c),
				Actual:   fmt.Sprintf("%.1f mm", bike.ForkA2c),
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

	statedStaticA2c := bike.ForkA2c
	if statedStaticA2c <= 0 && bike.ForkTravelMm > 0 {
		statedStaticA2c = bike.ForkTravelMm + 412.0
	}

	sagCompression := 0.0
	if bike.SagPercentagePublished > 0 && bike.ForkTravelMm > 0 {
		sagCompression = bike.ForkTravelMm * (bike.SagPercentagePublished / 100.0)
	}
	expectedForkA2c := geometry.Round(statedStaticA2c-sagCompression, 1)

	inferredMode := model.ModeUnspecified
	var a2cDiff *float64
	a2cSlop := slop.GetA2c()

	if expectedForkA2c > 0 {
		diff := geometry.Round(expectedForkA2c-calculatedForkA2c, 1)
		a2cDiff = &diff

		if math.Abs(diff) <= a2cSlop {
			if bike.SagPercentagePublished > 0 {
				inferredMode = model.ModeSaggedCad
				issues = append(issues, model.ValidationIssue{
					ID:       "stack_sagged_confirmed",
					Severity: model.SeveritySuccess,
					Title:    "Stack & Sagged Fork A2C Match",
					Message:  fmt.Sprintf("Geometric A2C (%.1fmm) matches published %.0f%% sagged fork A2C (%.1fmm) within %.1fmm (tolerance ±%.1fmm, %.1fmm CAD spacer).", calculatedForkA2c, bike.SagPercentagePublished, expectedForkA2c, math.Abs(diff), a2cSlop, geoSpacerMm),
					Expected: fmt.Sprintf("%.1f mm", expectedForkA2c),
					Actual:   fmt.Sprintf("%.1f mm", calculatedForkA2c),
				})
			} else {
				inferredMode = model.ModeStaticCad
				issues = append(issues, model.ValidationIssue{
					ID:       "stack_static_confirmed",
					Severity: model.SeveritySuccess,
					Title:    "Stack & Fork A2C Match (Static CAD)",
					Message:  fmt.Sprintf("Geometric A2C (%.1fmm) matches stated static fork A2C (%.1fmm) within %.1fmm (tolerance ±%.1fmm, %.1fmm CAD spacer).", calculatedForkA2c, expectedForkA2c, math.Abs(diff), a2cSlop, geoSpacerMm),
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

	sizes := frame.GetSizes()
	if len(sizes) == 0 || len(frame.Configurations) == 0 {
		report.IsValid = false
		report.Issues = append(report.Issues, model.ValidationIssue{
			ID:       "missing_configurations",
			Severity: model.SeverityError,
			Title:    "Missing Frame Configurations or Sizes",
			Message:  "Frame must define at least one size and one baseline configuration.",
		})
		return report
	}

	// Check adjustable chainstay boundaries
	if frame.HasAdjustableChainstay() {
		minCs := frame.GetMinChainstay()
		maxCs := frame.GetMaxChainstay()
		for _, cfg := range frame.Configurations {
			for sz, geo := range cfg.Geometries {
				if geo.ChainstayLength > 0 {
					if geo.ChainstayLength < minCs || geo.ChainstayLength > maxCs {
						report.IsValid = false
						report.Issues = append(report.Issues, model.ValidationIssue{
							ID:       "chainstay_out_of_range",
							Severity: model.SeverityError,
							Title:    "Chainstay Outside Adjustable Range",
							Message:  fmt.Sprintf("Published chainstay %.1fmm for size %s (%s) is outside legal range [%.1fmm, %.1fmm].", geo.ChainstayLength, sz, cfg.ID, minCs, maxCs),
						})
					}
				}
			}
		}
	}

	// Validate that all supported suspension travels resolve to plausible physical A2C across plausible sag ranges (0% to 30%)
	for _, travelMm := range frame.SupportedTravelMm {
		if travelMm <= 0 {
			continue
		}
		var staticA2c float64
		foundInConfig := false
		for _, cfg := range frame.Configurations {
			if math.Abs(cfg.Fork.TravelMm-travelMm) < 0.1 && cfg.Fork.A2c > 0 {
				staticA2c = cfg.Fork.A2c
				foundInConfig = true
				break
			}
		}
		if !foundInConfig {
			staticA2c = travelMm + 410.0
		}

		minStatic := travelMm + 400.0
		maxStatic := travelMm + 425.0
		if staticA2c < minStatic || staticA2c > maxStatic {
			report.IsValid = false
			report.Issues = append(report.Issues, model.ValidationIssue{
				ID:       "implausible_supported_travel_a2c",
				Severity: model.SeverityError,
				Title:    fmt.Sprintf("Implausible Supported Fork Travel A2C (%.0fmm)", travelMm),
				Message:  fmt.Sprintf("Supported travel %.0fmm resolves to static A2C %.1fmm, outside plausible 29er physical range [%.1fmm, %.1fmm].", travelMm, staticA2c, minStatic, maxStatic),
				Expected: fmt.Sprintf("%.1f - %.1f mm", minStatic, maxStatic),
				Actual:   fmt.Sprintf("%.1f mm", staticA2c),
			})
			continue
		}

		// Verify effective A2C across plausible sag range (0% to 30%)
		for _, sagPct := range []float64{0, 20, 30} {
			effA2c := geometry.Round(staticA2c-travelMm*(sagPct/100.0), 1)
			minEff := travelMm*0.70 + 400.0
			if effA2c < minEff || effA2c > maxStatic {
				report.IsValid = false
				report.Issues = append(report.Issues, model.ValidationIssue{
					ID:       "implausible_supported_travel_a2c",
					Severity: model.SeverityError,
					Title:    fmt.Sprintf("Implausible Effective A2C for %.0fmm at %.0f%% Sag", travelMm, sagPct),
					Message:  fmt.Sprintf("Supported travel %.0fmm at %.0f%% sag resolves to effective A2C %.1fmm (must be >= %.1fmm).", travelMm, sagPct, effA2c, minEff),
					Expected: fmt.Sprintf(">= %.1f mm", minEff),
					Actual:   fmt.Sprintf("%.1f mm", effA2c),
				})
				break
			}
		}
	}

	baseCfg := frame.GetBaselineConfiguration()
	for _, cfg := range frame.Configurations {
		for _, sz := range sizes {
			bike, err := geometry.MaterializeBikeGeometry(frame, sz, cfg.ID)
			if err != nil {
				report.IsValid = false
				report.Issues = append(report.Issues, model.ValidationIssue{
					ID:       "materialize_error",
					Severity: model.SeverityError,
					Title:    fmt.Sprintf("Failed to Materialize Size %s (%s)", sz, cfg.ID),
					Message:  err.Error(),
				})
				continue
			}
			valRep := ValidateBikeGeometry(bike)
			key := sz
			if baseCfg != nil && cfg.ID != baseCfg.ID {
				key = fmt.Sprintf("%s@%s", sz, cfg.ID)
			}
			report.SizeReports[key] = valRep
			if !valRep.IsValid {
				report.IsValid = false
			}
		}
	}

	if len(frame.Configurations) > 1 && baseCfg != nil {
		for _, cfg := range frame.Configurations {
			if cfg.ID == baseCfg.ID {
				continue
			}
			for _, sz := range sizes {
				cc := ValidateCrossConfig(frame, sz, cfg.ID)
				if cc != nil {
					report.CrossConfigs = append(report.CrossConfigs, *cc)
					if !cc.IsCoherent {
						report.IsValid = false
					}
				}
			}
		}
	}

	return report
}

// ValidateCrossConfig compares an alternate configuration's published geometry against the
// baseline configuration's expected kinematic pitching.
func ValidateCrossConfig(frame model.FrameModel, size string, comparedConfigID string) *model.CrossConfigValidation {
	if size == "" {
		size = frame.GetDefaultSize()
	}
	baseConfig := frame.GetBaselineConfiguration()
	compConfig := frame.GetConfiguration(comparedConfigID)
	if baseConfig == nil || compConfig == nil || baseConfig.ID == compConfig.ID {
		return nil
	}

	geoSpacer := frame.GetGeoHeadsetSpacerMm()
	deltaA2c := compConfig.Fork.A2c - baseConfig.Fork.A2c

	baseGeo, hasBase := baseConfig.Geometries[size]
	if !hasBase {
		return nil
	}

	publishedGeo, hasPublished := compConfig.Geometries[size]
	if !hasPublished || publishedGeo.Reach == 0 || publishedGeo.Stack == 0 {
		return &model.CrossConfigValidation{
			FrameModelID:     frame.ID,
			Size:             size,
			BaselineConfigID: baseConfig.ID,
			ComparedConfigID: compConfig.ID,
			BaselineA2c:      baseConfig.Fork.A2c,
			ComparedA2c:      compConfig.Fork.A2c,
			DeltaA2c:         deltaA2c,
			IsCoherent:       true,
			Message:          "Configuration is dynamically derived from baseline via kinematic rotation (no explicit manual overrides).",
		}
	}

	baseStructural := baseConfig.Fork.A2c + geoSpacer
	compStructural := compConfig.Fork.A2c + geoSpacer
	dyn := geometry.CalculateDynamicGeometry(
		baseGeo.Reach,
		baseGeo.Stack,
		baseGeo.HtaDeg,
		baseGeo.StaDeg,
		baseGeo.BbDrop,
		baseStructural,
		compStructural,
		baseGeo.Wheelbase,
		baseGeo.ChainstayLength,
	)

	var diffs []model.CrossConfigDiff
	maxDisc := 0.0

	checkMetric := func(name string, baseVal, pubVal, kinVal float64, isAngle bool) {
		pubDiff := geometry.Round(pubVal-baseVal, 2)
		kinDiff := geometry.Round(kinVal-baseVal, 2)
		discrepancy := geometry.Round(pubVal-kinVal, 2)
		absDisc := math.Abs(discrepancy)
		if absDisc > maxDisc {
			maxDisc = absDisc
		}
		diffs = append(diffs, model.CrossConfigDiff{
			Metric:        name,
			BaselineValue: baseVal,
			ConfigValue:   pubVal,
			PublishedDiff: pubDiff,
			KinematicDiff: kinDiff,
			Discrepancy:   discrepancy,
		})
	}

	checkMetric("Head Tube Angle (°)", baseGeo.HtaDeg, publishedGeo.HtaDeg, dyn.NewHta, true)
	checkMetric("Seat Tube Angle (°)", baseGeo.StaDeg, publishedGeo.StaDeg, dyn.NewSta, true)
	checkMetric("Reach (mm)", baseGeo.Reach, publishedGeo.Reach, dyn.NewReach, false)
	checkMetric("Stack (mm)", baseGeo.Stack, publishedGeo.Stack, dyn.NewStack, false)
	checkMetric("BB Drop (mm)", baseGeo.BbDrop, publishedGeo.BbDrop, dyn.NewBbDrop, false)

	ccSlop := frame.GetGeoSlopMm().GetCrossConfig()
	isCoherent := maxDisc <= ccSlop
	message := "Published configuration is physically coherent with baseline frame rotation."
	if !isCoherent {
		message = fmt.Sprintf("CAD discrepancy detected (max delta: %.1f > %.1fmm tolerance): published numbers deviate from kinematic 2D rotation.", maxDisc, ccSlop)
	}

	return &model.CrossConfigValidation{
		FrameModelID:     frame.ID,
		Size:             size,
		BaselineConfigID: baseConfig.ID,
		ComparedConfigID: compConfig.ID,
		BaselineA2c:      baseConfig.Fork.A2c,
		ComparedA2c:      compConfig.Fork.A2c,
		DeltaA2c:         deltaA2c,
		IsCoherent:       isCoherent,
		MaxDiscrepancy:   maxDisc,
		Diffs:            diffs,
		Message:          message,
	}
}

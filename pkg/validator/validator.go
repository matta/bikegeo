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

	hasError := false
	for _, issue := range issues {
		if issue.Severity == model.SeverityError {
			hasError = true
			break
		}
	}

	effectiveCupStack := bike.HeadsetCupType.CupStackMm()

	if hasError {
		return model.ValidationReport{
			BikeID:              bike.ID,
			IsValid:             false,
			Issues:              issues,
			GeometricA2c:        0,
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

	// 3. Headset Cup Type Validation
	if !bike.HeadsetCupType.IsValid() {
		issues = append(issues, model.ValidationIssue{
			ID:       "invalid_headset_cup_type",
			Field:    "headset_cup_type",
			Severity: model.SeverityError,
			Title:    "Unrecognized Headset Cup Type",
			Message:  fmt.Sprintf("Headset cup type %q is not a recognized SHIS standard (EC44, ZS44, ZS56, IS42, IS52).", bike.HeadsetCupType),
			Expected: "EC44, ZS44, ZS56, IS42, or IS52",
			Actual:   string(bike.HeadsetCupType),
		})
	}

	// 4. Reverse-Engineering Geometric Axle-to-Crown (Master Stack Equation Check)
	structuralA2c, calculatedForkA2c := geometry.CalculateGeometricA2c(
		bike.Stack,
		bike.BbDrop,
		bike.Htl,
		bike.HtaDeg,
		bike.ForkRake,
		effectiveCupStack,
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

	if expectedForkA2c > 0 {
		diffWithCup := geometry.Round(expectedForkA2c-calculatedForkA2c, 1)
		diffWithoutCup := geometry.Round(expectedForkA2c-structuralA2c, 1)

		if math.Abs(diffWithCup) <= 6.0 {
			a2cDiff = &diffWithCup
			if bike.SagPercentagePublished > 0 {
				inferredMode = model.ModeSaggedCad
				issues = append(issues, model.ValidationIssue{
					ID:       "stack_sagged_confirmed",
					Severity: model.SeveritySuccess,
					Title:    "Stack & Sagged Fork A2C Match",
					Message:  fmt.Sprintf("Geometric A2C (%.1fmm) matches published %.0f%% sagged fork A2C (%.1fmm) within %.1fmm.", calculatedForkA2c, bike.SagPercentagePublished, expectedForkA2c, math.Abs(diffWithCup)),
					Expected: fmt.Sprintf("%.1f mm", expectedForkA2c),
					Actual:   fmt.Sprintf("%.1f mm", calculatedForkA2c),
				})
			} else {
				inferredMode = model.ModeStaticCad
				issues = append(issues, model.ValidationIssue{
					ID:       "stack_static_confirmed",
					Severity: model.SeveritySuccess,
					Title:    "Stack & Fork A2C Match (Static CAD)",
					Message:  fmt.Sprintf("Geometric A2C (%.1fmm) matches stated static fork A2C (%.1fmm) within %.1fmm.", calculatedForkA2c, expectedForkA2c, math.Abs(diffWithCup)),
					Details:  "The manufacturer published geometry measured with an unweighted (0% sag) fork and lower headset cup included.",
					Expected: fmt.Sprintf("%.1f mm", expectedForkA2c),
					Actual:   fmt.Sprintf("%.1f mm", calculatedForkA2c),
				})
			}
		} else if effectiveCupStack > 0 && math.Abs(diffWithoutCup) <= 6.0 {
			a2cDiff = &diffWithoutCup
			inferredMode = model.ModeCupOmittedCad
			issues = append(issues, model.ValidationIssue{
				ID:       "stack_cup_omitted_cad",
				Severity: model.SeverityInfo,
				Title:    "CAD Chart Omitted External Lower Headset Cup",
				Message:  fmt.Sprintf("CAD structural A2C (%.1fmm) matches stated fork A2C (%.1fmm) within %.1fmm (0mm lower cup in CAD; %s adds +%.0fmm physically).", structuralA2c, expectedForkA2c, math.Abs(diffWithoutCup), bike.HeadsetCupType, effectiveCupStack),
				Details:  "The manufacturer calculated published Stack without the external lower headset cup height; canonical comparisons automatically add the physical cup stack.",
				Expected: fmt.Sprintf("%.1f mm (bare fork)", expectedForkA2c),
				Actual:   fmt.Sprintf("%.1f mm (CAD total)", structuralA2c),
			})
		} else {
			a2cDiff = &diffWithCup
			inferredMode = model.ModeMismatchedCad
			issues = append(issues, model.ValidationIssue{
				ID:         "stack_a2c_mismatch",
				Field:      "stack",
				Severity:   model.SeverityError,
				Title:      "Geometric Inconsistency in Stack / A2C",
				Message:    fmt.Sprintf("The geometric A2C required to produce Stack %.1fmm is %.1fmm, diverging from stated fork %.1fmm by %.1fmm.", bike.Stack, calculatedForkA2c, expectedForkA2c, diffWithCup),
				Details:    "Published Stack, HTL, BB Drop, HTA, and Fork A2C do not satisfy the Master Stack Equation within ±6mm tolerance.",
				Expected:   fmt.Sprintf("%.1f mm", expectedForkA2c),
				Actual:     fmt.Sprintf("%.1f mm", calculatedForkA2c),
				Difference: diffWithCup,
			})
		}
	}

	// 5. Front-Center & Wheelbase Consistency Check
	calculatedFc := geometry.Round(
		geometry.CalculateFrontCenter(
			bike.Reach,
			bike.Htl,
			bike.HtaDeg,
			bike.ForkRake,
			structuralA2c,
		),
		1,
	)

	var fcDiff *float64
	if bike.FrontCenter > 0 {
		diff := geometry.Round(math.Abs(bike.FrontCenter-calculatedFc), 1)
		fcDiff = &diff
		if diff > 12.0 {
			issues = append(issues, model.ValidationIssue{
				ID:         "front_center_mismatch",
				Field:      "front_center",
				Severity:   model.SeverityError,
				Title:      "Front Center Inconsistency",
				Message:    fmt.Sprintf("Published Front Center (%.1fmm) differs from geometrically calculated FC (%.1fmm) by %.1fmm.", bike.FrontCenter, calculatedFc, diff),
				Details:    "Check if Reach, HTA, or Fork Rake is inconsistent in published geometry.",
				Expected:   fmt.Sprintf("%.1f mm (calculated)", calculatedFc),
				Actual:     fmt.Sprintf("%.1f mm (published)", bike.FrontCenter),
				Difference: diff,
			})
		} else {
			issues = append(issues, model.ValidationIssue{
				ID:       "front_center_ok",
				Severity: model.SeveritySuccess,
				Title:    "Front Center Geometrically Coherent",
				Message:  fmt.Sprintf("Published Front Center matches calculated 2D vector within %.1fmm.", diff),
			})
		}
	} else if bike.Wheelbase > 0 && bike.ChainstayLength > 0 {
		csHorizontal := math.Sqrt(math.Max(0, bike.ChainstayLength*bike.ChainstayLength-bike.BbDrop*bike.BbDrop))
		estimatedWb := geometry.Round(csHorizontal+calculatedFc, 1)
		wbDiff := geometry.Round(math.Abs(bike.Wheelbase-estimatedWb), 1)

		if wbDiff > 15.0 {
			issues = append(issues, model.ValidationIssue{
				ID:       "wheelbase_discrepancy",
				Field:    "wheelbase",
				Severity: model.SeverityError,
				Title:    "Wheelbase & Chainstay Discrepancy",
				Message:  fmt.Sprintf("Wheelbase (%.1fmm) does not match Rear Center (%.1fmm) + Front Center (%.1fmm) = %.1fmm (Δ %.1fmm).", bike.Wheelbase, csHorizontal, calculatedFc, estimatedWb, wbDiff),
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
		CupStack:            effectiveCupStack,
		TotalStructuralA2c:  geometry.Round(structuralA2c, 1),
		A2cDifference:       a2cDiff,
		InferredPublishMode: inferredMode,
		FrontCenterCalc:     calculatedFc,
		FrontCenterDiff:     fcDiff,
	}
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

	if !frame.HeadsetCupType.IsValid() {
		report.IsValid = false
		report.Issues = append(report.Issues, model.ValidationIssue{
			ID:       "invalid_headset_cup_type",
			Field:    "headset_cup_type",
			Severity: model.SeverityError,
			Title:    "Unrecognized Headset Cup Type",
			Message:  fmt.Sprintf("Headset cup type %q is not a recognized SHIS standard (EC44, ZS44, ZS56, IS42, IS52).", frame.HeadsetCupType),
		})
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

	cupStack := frame.HeadsetCupType.CupStackMm()
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

	baseStructural := baseConfig.Fork.A2c + cupStack
	compStructural := compConfig.Fork.A2c + cupStack
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

	isCoherent := maxDisc <= 3.0
	message := "Published configuration is physically coherent with baseline frame rotation."
	if !isCoherent {
		message = fmt.Sprintf("CAD discrepancy detected (max delta: %.1f): published numbers deviate from kinematic 2D rotation.", maxDisc)
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


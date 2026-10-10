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

	hasError := false
	for _, issue := range issues {
		if issue.Severity == model.SeverityError {
			hasError = true
			break
		}
	}

	effectiveCupStack := bike.CupStack
	if effectiveCupStack == 0 && bike.HeadsetCupType == model.CupEC44 {
		effectiveCupStack = 12.0
	}

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
			Severity: model.SeverityWarning,
			Title:    "Unusual Head Tube Angle",
			Message:  fmt.Sprintf("HTA of %.1f° is outside normal range (58° - 74°).", bike.HtaDeg),
			Actual:   fmt.Sprintf("%.1f°", bike.HtaDeg),
		})
	}

	if bike.StaDeg < 65.0 || bike.StaDeg > 82.0 {
		issues = append(issues, model.ValidationIssue{
			ID:       "sta_range",
			Field:    "sta_deg",
			Severity: model.SeverityWarning,
			Title:    "Unusual Seat Tube Angle",
			Message:  fmt.Sprintf("STA of %.1f° is outside typical range (70° - 79°).", bike.StaDeg),
			Actual:   fmt.Sprintf("%.1f°", bike.StaDeg),
		})
	}

	if bike.BbDrop < 15.0 || bike.BbDrop > 95.0 {
		issues = append(issues, model.ValidationIssue{
			ID:       "bb_drop_range",
			Field:    "bb_drop",
			Severity: model.SeverityWarning,
			Title:    "Atypical Bottom Bracket Drop",
			Message:  fmt.Sprintf("BB drop of %.1fmm is outside typical mountain bike range (30mm - 80mm).", bike.BbDrop),
			Actual:   fmt.Sprintf("%.1f mm", bike.BbDrop),
		})
	}

	// 3. Headset Cup & Lower Stack Reality Check (EC44 check)
	isStraight44 := bike.HeadsetCupType == model.CupEC44
	if isStraight44 && (bike.CupStack == 0 || bike.CupStack < 10.0) {
		issues = append(issues, model.ValidationIssue{
			ID:       "headset_ec44_missing_stack",
			Field:    "cup_stack",
			Severity: model.SeverityWarning,
			Title:    "EC44 Lower Cup Stack Notice",
			Message:  "Frames with straight 44mm head tubes running tapered forks require an EC44 external lower cup.",
			Details:  "Standard EC44/40 cups add ~12mm of structural height below the head tube. If not accounted for in CAD or build specs, the bike will sit slacker and taller than stated.",
			Expected: "12 mm",
			Actual:   fmt.Sprintf("%.1f mm", effectiveCupStack),
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

	inferredMode := model.ModeUnspecified
	var a2cDiff *float64

	if bike.ForkA2c > 0 {
		diff := geometry.Round(bike.ForkA2c-calculatedForkA2c, 1)
		a2cDiff = &diff

		if math.Abs(diff) <= 6.0 {
			inferredMode = model.ModeStaticCad
			issues = append(issues, model.ValidationIssue{
				ID:       "stack_static_confirmed",
				Severity: model.SeveritySuccess,
				Title:    "Stack & Fork A2C Match (Static CAD)",
				Message:  fmt.Sprintf("Geometric A2C (%.1fmm) matches stated static fork A2C (%.1fmm) within %.1fmm.", calculatedForkA2c, bike.ForkA2c, math.Abs(diff)),
				Details:  "The manufacturer published geometry measured with an unweighted (0% sag) fork.",
				Expected: fmt.Sprintf("%.1f mm", bike.ForkA2c),
				Actual:   fmt.Sprintf("%.1f mm", calculatedForkA2c),
			})
		} else if diff >= 15.0 && diff <= 38.0 {
			inferredMode = model.ModeSaggedCad
			sagEstPercent := 20.0
			if bike.ForkTravelMm > 0 {
				sagEstPercent = geometry.Round((diff/bike.ForkTravelMm)*100.0, 0)
			} else {
				sagEstPercent = geometry.Round((diff/120.0)*100.0, 0)
			}
			issues = append(issues, model.ValidationIssue{
				ID:         "sagged_cad_detected",
				Severity:   model.SeverityInfo,
				Title:      "Hardtail Sagged CAD Convention Detected",
				Message:    fmt.Sprintf("Published Stack corresponds to a sagged fork (geometric A2C = %.1fmm, ~%.1fmm compression).", calculatedForkA2c, diff),
				Details:    fmt.Sprintf("The manufacturer published numbers with approximately %.0f%% fork sag rather than static fork length (%.1fmm). Canonicalizing will convert this to apples-to-apples.", sagEstPercent, bike.ForkA2c),
				Expected:   fmt.Sprintf("%.1f mm (static)", bike.ForkA2c),
				Actual:     fmt.Sprintf("%.1f mm (sagged CAD)", calculatedForkA2c),
				Difference: diff,
			})
		} else {
			inferredMode = model.ModeMismatchedCad
			issues = append(issues, model.ValidationIssue{
				ID:         "stack_a2c_mismatch",
				Field:      "stack",
				Severity:   model.SeverityWarning,
				Title:      "Geometric Inconsistency in Stack / A2C",
				Message:    fmt.Sprintf("The geometric A2C required to produce Stack %.1fmm is %.1fmm, diverging from stated fork %.1fmm by %.1fmm.", bike.Stack, calculatedForkA2c, bike.ForkA2c, diff),
				Details:    "Possible causes: typo in published Stack or HTL, unlisted headset cup height, or chart calculated under different wheel diameter/sag assumptions.",
				Expected:   fmt.Sprintf("%.1f mm", bike.ForkA2c),
				Actual:     fmt.Sprintf("%.1f mm", calculatedForkA2c),
				Difference: diff,
			})
		}
	} else if bike.ForkTravelMm > 0 {
		estimatedStaticA2c := bike.ForkTravelMm + 412.0
		diffFromEstimated := geometry.Round(estimatedStaticA2c-calculatedForkA2c, 1)

		if math.Abs(diffFromEstimated) <= 8.0 {
			inferredMode = model.ModeStaticCad
			issues = append(issues, model.ValidationIssue{
				ID:       "estimated_static_match",
				Severity: model.SeverityInfo,
				Title:    "Consistent with Standard Static Fork Travel",
				Message:  fmt.Sprintf("Geometric A2C (%.1fmm) aligns with typical %.0fmm 29er fork (~%.0fmm static).", calculatedForkA2c, bike.ForkTravelMm, estimatedStaticA2c),
			})
		} else if diffFromEstimated >= 15.0 && diffFromEstimated <= 38.0 {
			inferredMode = model.ModeSaggedCad
			issues = append(issues, model.ValidationIssue{
				ID:       "estimated_sagged_match",
				Severity: model.SeverityInfo,
				Title:    "Probable Sagged Geometry Chart",
				Message:  fmt.Sprintf("Geometric A2C (%.1fmm) is ~%.1fmm shorter than a static %.0fmm fork, indicating sagged CAD baseline.", calculatedForkA2c, diffFromEstimated, bike.ForkTravelMm),
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
				Severity:   model.SeverityWarning,
				Title:      "Front Center Inconsistency",
				Message:    fmt.Sprintf("Published Front Center (%.1fmm) differs from geometrically calculated FC (%.1fmm) by %.1fmm.", bike.FrontCenter, calculatedFc, diff),
				Details:    "Check if Reach or Fork Rake was modified in published geometry.",
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
				Severity: model.SeverityWarning,
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
		StatedA2c:           bike.ForkA2c,
		CupStack:            effectiveCupStack,
		TotalStructuralA2c:  geometry.Round(structuralA2c, 1),
		A2cDifference:       a2cDiff,
		InferredPublishMode: inferredMode,
		FrontCenterCalc:     calculatedFc,
		FrontCenterDiff:     fcDiff,
	}
}

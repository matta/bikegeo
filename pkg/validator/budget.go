package validator

import (
	"fmt"
	"strings"

	"bikegeo/pkg/model"
)

// budgetSummary renders an ErrorBudget for issue messages, for example
// "rounding budget ±2.9mm: lengths ±1.6 + HTA ±1.3, 59% used".
func budgetSummary(b model.ErrorBudget) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "rounding budget ±%.1fmm: lengths ±%.1f", b.RoundingTol(), b.LengthTol)
	if b.AngleTol > 0 {
		fmt.Fprintf(&sb, " + HTA ±%.1f", b.AngleTol)
	}
	fmt.Fprintf(&sb, ", %.0f%% used", 100*b.Saturation())
	if b.Allowance > 0 {
		fmt.Fprintf(&sb, "; declared allowance +%.1fmm", b.Allowance)
	}
	return sb.String()
}

// formatBudgetTerm renders one input's contribution, for example "HTA (±0.25°) ±1.3mm".
func formatBudgetTerm(t model.BudgetTerm) string {
	if t.IsAngle {
		return fmt.Sprintf("%s (±%g°) ±%.2fmm", t.Name, t.HalfWidth, t.ContributionMm)
	}
	return fmt.Sprintf("%s (±%gmm) ±%.2fmm", t.Name, t.HalfWidth, t.ContributionMm)
}

// topBudgetTerms lists the n largest contributors to a budget.
func topBudgetTerms(b model.ErrorBudget, n int) string {
	var parts []string
	for i, t := range b.Terms {
		if i == n {
			break
		}
		parts = append(parts, formatBudgetTerm(t))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

// budgetTierNotice returns an informational issue for a passing check that is
// explained only by assuming the head angle is rounded, or only by the frame's
// declared allowance. Checks explained by length rounding alone need no notice.
func budgetTierNotice(id, field, label string, b model.ErrorBudget) (model.ValidationIssue, bool) {
	switch b.Tier() {
	case model.TierAngleRounding:
		needed := 0.0
		if b.AngleTol > 0 {
			needed = (abs(b.Residual) - b.LengthTol) / b.AngleTol
		}
		return model.ValidationIssue{
			ID:       id + "_needs_angle_rounding",
			Field:    field,
			Severity: model.SeverityInfo,
			Title:    label + " Relies on Head Angle Rounding",
			Message: fmt.Sprintf("The %.1fmm residual exceeds the ±%.1fmm that rounding of the published lengths can explain; it fits only if the published head angle is itself rounded (using %.0f%% of its ±%.1fmm effect).",
				abs(b.Residual), b.LengthTol, 100*needed, b.AngleTol),
			Details: "Rounding alone could explain this, but so could a small modelling error (rake, spacer, sag); a chart with more precise angles would settle it.",
		}, true
	case model.TierAllowance:
		return model.ValidationIssue{
			ID:       id + "_uses_allowance",
			Field:    field,
			Severity: model.SeverityInfo,
			Title:    label + " Uses Declared Allowance",
			Message: fmt.Sprintf("The %.1fmm residual exceeds the ±%.1fmm rounding budget by %.1fmm and passes only with the declared geo_slop_mm allowance (+%.1fmm).",
				abs(b.Residual), b.RoundingTol(), b.NeededAllowance(), b.Allowance),
		}, true
	case model.TierLengthRounding, model.TierExceeds:
		// Length rounding needs no notice; a failure is reported as an error elsewhere.
	}
	return model.ValidationIssue{}, false
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

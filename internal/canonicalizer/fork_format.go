package canonicalizer

import (
	"fmt"
	"math"
	"strings"

	"bikegeo/internal/model"
)

// FormatRigidFork produces the canonical display label for a rigid fork with the given A2C.
func FormatRigidFork(a2c float64) string {
	return fmt.Sprintf("Rigid (%.0fmm)", math.Round(a2c))
}

// FormatSuspensionFork produces the canonical display label for a suspension fork with travel and sag.
func FormatSuspensionFork(travelMm, sagPct float64) string {
	return fmt.Sprintf("%.0fmm @ %.0f%% sag", travelMm, sagPct)
}

// FormatCustomA2cFork produces the display label for an explicit A2C/rake fork specification.
func FormatCustomA2cFork(a2cMm, rakeMm float64) string {
	return fmt.Sprintf("a2c=%.0f/%.0f", a2cMm, rakeMm)
}

// FormatStockFork produces the display label for a default/stock fork configuration.
func FormatStockFork() string {
	return "Stock"
}

// GetEffectiveForkLabel returns the primary display label for a row, falling back to its Setup label.
func GetEffectiveForkLabel(r model.CanonicalComparisonRow) string {
	if r.DisplayLabel != "" {
		return r.DisplayLabel
	}
	return r.Setup.DisplayLabel
}

// FormatForkEffA2c formats the "Fork & Eff A2C" column display value in comparison matrices.
// For rigid forks, it avoids redundant repetitions and merely states "Rigid (<A2C>mm)".
// For suspension forks, it appends the effective (sagged) A2C: "<Fork> (<EffA2C>mm)".
func FormatForkEffA2c(r model.CanonicalComparisonRow) string {
	if r.Setup.IsRigid {
		return FormatRigidFork(r.EffectiveA2c)
	}
	lbl := GetEffectiveForkLabel(r)
	if lbl == "" {
		return fmt.Sprintf("%.0fmm A2C", math.Round(r.EffectiveA2c))
	}
	effStr := fmt.Sprintf("(%.0fmm)", math.Round(r.EffectiveA2c))
	if strings.Contains(lbl, effStr) {
		return lbl
	}
	return fmt.Sprintf("%s %s", lbl, effStr)
}

// FormatForkType extracts a short fork type identifier (e.g. "rigid", "120mm") for frame name disambiguation.
func FormatForkType(r model.CanonicalComparisonRow) string {
	if r.Setup.IsRigid {
		return "rigid"
	}
	if r.Setup.TravelMm > 0 {
		return fmt.Sprintf("%.0fmm", r.Setup.TravelMm)
	}
	lbl := GetEffectiveForkLabel(r)
	if lbl != "" {
		return lbl
	}
	return fmt.Sprintf("%.0fmm A2C", math.Round(r.EffectiveA2c))
}

// FormatForkSag extracts a fork+sag identifier (e.g. "rigid", "120mm @ 20% sag") for frame name disambiguation.
func FormatForkSag(r model.CanonicalComparisonRow) string {
	if r.Setup.IsRigid {
		return "rigid"
	}
	lbl := GetEffectiveForkLabel(r)
	clean := strings.TrimSpace(lbl)
	if clean != "" {
		return clean
	}
	return fmt.Sprintf("%.0fmm A2C", math.Round(r.EffectiveA2c))
}

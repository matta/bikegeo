package ideal

import (
	"fmt"

	"bikegeo/pkg/model"
)

// AttributeMeta contains metadata and extraction logic for a known bike geometry or fit attribute.
type AttributeMeta struct {
	ID        MetricID
	Label     string
	Unit      string
	Decimals  int
	BelowWord string
	AboveWord string
	Extract   func(row model.CanonicalComparisonRow) (float64, bool)
	FormatVal func(val float64, row *model.CanonicalComparisonRow) string
}

var registry = []AttributeMeta{
	// Deprecated: setback measures horizontal distance to the Seat Axis Point (H * cos(STA)),
	// ignoring seatpost setback and saddle rails. Use brp_setback for BRP-referenced saddle setback.
	{
		ID:        "setback",
		Label:     "Seat Axis Setback",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "fwd",
		AboveWord: "aft",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.ActualSetback, true
		},
	},
	// ett is the industry-standard effective top tube: head-tube top center back to the seat-tube axis at stack height.
	{
		ID:        "ett",
		Label:     "ETT",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "sht",
		AboveWord: "lng",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.Ett, row.Canonical.Ett > 0
		},
	},
	// aett measures top-tube reach from the Seat Axis Point at height H (frame-only; independent of saddle, stem, bar).
	{
		ID:        "aett",
		Label:     "AETT",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "sht",
		AboveWord: "lng",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.Aett, true
		},
	},
	{
		ID:        "sparevert",
		Label:     "Seat Axis–HT Drop",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "lo",
		AboveWord: "hi",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.SpareVerticalStack, true
		},
	},
	{
		ID:        "spareaxial",
		Label:     "Spare Axial",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "lo",
		AboveWord: "hi",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.SpareAxialSteerer, true
		},
	},
	{
		ID:        "seatpost_extension",
		Label:     "Seatpost Ext",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "sht",
		AboveWord: "lng",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			if row.Canonical.SeatpostExtensionMm > 0 {
				return row.Canonical.SeatpostExtensionMm, true
			}
			if row.Baseline.SeatpostExtension > 0 {
				return row.Baseline.SeatpostExtension, true
			}
			return 0, false
		},
	},
	// Deprecated: seat_to_rc (Seat Axis–Rear Axle) only measures horizontal distance from rear axle to the Seat Axis Point, ignoring front-center.
	// Use rc_wb_ratio for rear-center-to-wheelbase proportion.
	{
		ID:        "seat_to_rc",
		Label:     "Seat Axis–Rear Axle",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "sht",
		AboveWord: "lng",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.SeatToRearCenter, true
		},
	},
	{
		ID:        "chainstay",
		Label:     "Chainstay",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "sht",
		AboveWord: "lng",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.ActiveChainstay, true
		},
		FormatVal: func(val float64, row *model.CanonicalComparisonRow) string {
			star := ""
			if row != nil && row.Bike.HasAdjustableChainstay() {
				star = "*"
			}
			return fmt.Sprintf("%.0f%s", val, star)
		},
	},
	{
		ID:        "bbdrop",
		Label:     "BB Drop",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "hi",
		AboveWord: "lo",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.ActiveBbDrop, true
		},
	},
	{
		ID:        "hta",
		Label:     "HTA",
		Unit:      "°",
		Decimals:  1,
		BelowWord: "slk",
		AboveWord: "stp",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return float64(row.Canonical.ActiveHta), true
		},
	},
	{
		ID:        "trail",
		Label:     "Trail",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "qck",
		AboveWord: "slw",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			if row.Canonical.TrailMm > 0 {
				return row.Canonical.TrailMm, true
			}
			if row.Baseline.TrailMm > 0 {
				return row.Baseline.TrailMm, true
			}
			return 0, false
		},
	},
	{
		ID:        "sta",
		Label:     "STA",
		Unit:      "°",
		Decimals:  1,
		BelowWord: "slk",
		AboveWord: "stp",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return float64(row.Canonical.ActiveSta), true
		},
	},
	{
		ID:        "reach",
		Label:     "Reach",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "sht",
		AboveWord: "lng",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.ActiveReach, true
		},
	},
	{
		ID:        "stack",
		Label:     "Stack",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "lo",
		AboveWord: "hi",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.ActiveStack, true
		},
	},
	{
		ID:        "wheelbase",
		Label:     "Wheelbase",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "sht",
		AboveWord: "lng",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.WheelbaseCalc, true
		},
	},
	{
		ID:       "rc_wb_ratio",
		Label:    "RC / WB",
		Unit:     "%",
		Decimals: 1,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.RearCenterWheelbaseRatio * 100.0, true
		},
	},
	{
		ID:        "seattube",
		Label:     "Seat Tube",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "sht",
		AboveWord: "lng",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			if row.Bike.SeatTubeLength <= 0 {
				return 0, false
			}
			return row.Bike.SeatTubeLength, true
		},
	},
	{
		ID:        "standover",
		Label:     "Standover",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "lo",
		AboveWord: "hi",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			if row.Bike.StandoverHeight <= 0 {
				return 0, false
			}
			return row.Bike.StandoverHeight, true
		},
	},
	{
		ID:        "brp_setback",
		Label:     "BRP Setback",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "fwd",
		AboveWord: "aft",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			brp := row.Canonical.DeadCenterBRP
			if row.Canonical.SelectedBRP != nil {
				brp = *row.Canonical.SelectedBRP
			}
			if brp.BrpSetbackMm <= 0 {
				return 0, false
			}
			return brp.BrpSetbackMm, true
		},
	},
	{
		ID:        "saddle_hb",
		Label:     "BRP–HandGrip",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "sht",
		AboveWord: "lng",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			if row.Canonical.SaddleHbMm <= 0 {
				return 0, false
			}
			return row.Canonical.SaddleHbMm, true
		},
	},
	{
		ID:        "saddle_hb_drop",
		Label:     "HandGrip Rise",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "lo",
		AboveWord: "hi",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			if row.Canonical.SelectedHB == nil {
				return 0, false
			}
			return row.Canonical.SelectedHB.SaddleHbDropMm, true
		},
	},
	{
		ID:       "best_stem",
		Label:    "Best Stem",
		Unit:     "",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			if row.Canonical.SelectedHB == nil || row.Canonical.SelectedHB.Stem.LengthMm <= 0 {
				return 0, false
			}
			return row.Canonical.SelectedHB.Stem.LengthMm, true
		},
		FormatVal: func(val float64, row *model.CanonicalComparisonRow) string {
			if row != nil && row.Canonical.SelectedHB != nil && row.Canonical.SelectedHB.Stem.LengthMm > 0 {
				hb := row.Canonical.SelectedHB
				sign := "+"
				if hb.SpacerMm < 0 {
					sign = ""
				}
				return fmt.Sprintf("%.0fmm %.0f° %s%.0fmm", hb.Stem.LengthMm, hb.Stem.AngleDeg, sign, hb.SpacerMm)
			}
			return fmt.Sprintf("%.0fmm", val)
		},
	},
	{
		ID:       "stem_spacer_fit",
		Label:    "Stem/Spacer Fit",
		Unit:     "",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			if row.Canonical.StemSpacerFit == nil {
				return 0, false
			}
			return row.Canonical.StemSpacerFit.StemLengthMm, true
		},
		FormatVal: func(val float64, row *model.CanonicalComparisonRow) string {
			if row != nil && row.Canonical.StemSpacerFit != nil {
				fit := row.Canonical.StemSpacerFit
				sign := "+"
				if fit.SpacerMm < 0 {
					sign = ""
				}
				return fmt.Sprintf("%.0fmm %.0f° %s%.0fmm", fit.StemLengthMm, fit.StemAngleDeg, sign, fit.SpacerMm)
			}
			return fmt.Sprintf("%.0fmm", val)
		},
	},
	{
		ID:        "sit_bone_setback",
		Label:     "Sit Bones",
		Unit:      "mm",
		Decimals:  0,
		BelowWord: "fwd",
		AboveWord: "aft",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			if row.Canonical.SitBoneSetbackMm <= 0 {
				return 0, false
			}
			return row.Canonical.SitBoneSetbackMm, true
		},
	},
	{
		ID:        "ideal_sta",
		Label:     "Ideal STA",
		Unit:      "°",
		Decimals:  1,
		BelowWord: "slk",
		AboveWord: "stp",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			if row.Canonical.IdealStaDeg <= 0 {
				return 0, false
			}
			return float64(row.Canonical.IdealStaDeg), true
		},
	},
	{
		ID:        "handlebar_clamp_fit",
		Label:     "Oval Fits",
		Unit:      "",
		Decimals:  0,
		BelowWord: "lo",
		AboveWord: "hi",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			if row.Canonical.CockpitFitResult == nil {
				return 0, false
			}
			return row.Canonical.CockpitFitResult.FinalSubScore, true
		},
		FormatVal: func(val float64, row *model.CanonicalComparisonRow) string {
			if row != nil && row.Canonical.CockpitFitResult != nil {
				return fmt.Sprintf("%d", row.Canonical.CockpitFitResult.CountInside)
			}
			return fmt.Sprintf("%.0f", val)
		},
	},
}

var registryMap = func() map[MetricID]AttributeMeta {
	m := make(map[MetricID]AttributeMeta, len(registry))
	for _, a := range registry {
		m[a.ID] = a
	}
	return m
}()

// LookupAttribute returns the AttributeMeta for a given MetricID, or false if unknown.
func LookupAttribute(id MetricID) (AttributeMeta, bool) {
	switch id {
	case MetricSetback,
		MetricEtt,
		MetricAett,
		MetricSpareVert,
		MetricSpareAxial,
		MetricSeatpostExtension,
		MetricSeatToRc,
		MetricChainstay,
		MetricBbDrop,
		MetricHta,
		MetricTrail,
		MetricSta,
		MetricReach,
		MetricStack,
		MetricWheelbase,
		MetricRcWbRatio,
		MetricSeatTube,
		MetricStandover,
		MetricBrpSetback,
		MetricSaddleHb,
		MetricSaddleHbDrop,
		MetricBestStem,
		MetricStemSpacerFit,
		MetricSitBoneSetback,
		MetricIdealSta,
		MetricHandlebarClampFit:
		meta, ok := registryMap[id]
		return meta, ok
	default:
		return AttributeMeta{}, false
	}
}

// AllAttributes returns a copy of all supported AttributeMeta definitions in standard order.
func AllAttributes() []AttributeMeta {
	copied := make([]AttributeMeta, len(registry))
	copy(copied, registry)
	return copied
}

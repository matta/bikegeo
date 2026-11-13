package ideal

import (
	"fmt"
	"strings"

	"bikegeo/pkg/model"
)

// AttributeMeta contains metadata and extraction logic for a known bike geometry or fit attribute.
type AttributeMeta struct {
	ID        string
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
	// Use frontwt for fore/aft rider weight distribution and wheel centering.
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
		ID:        "frontwt",
		Label:     "Front Wt",
		Unit:      "%",
		Decimals:  1,
		BelowWord: "lo",
		AboveWord: "hi",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.WeightDistFrontPct, true
		},
	},
	{
		ID:        "backangle",
		Label:     "Back Angle",
		Unit:      "°",
		Decimals:  1,
		BelowWord: "lo",
		AboveWord: "hi",
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return float64(row.Canonical.BackAngleDeg), true
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
		Label:     "BRP–HB",
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
		Label:     "HB Rise",
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

var registryMap = func() map[string]AttributeMeta {
	m := make(map[string]AttributeMeta, len(registry)*2)
	for _, a := range registry {
		lower := strings.ToLower(a.ID)
		m[lower] = a
		cleaned := strings.ReplaceAll(strings.ReplaceAll(lower, "_", ""), "-", "")
		if cleaned != lower {
			m[cleaned] = a
		}
	}
	return m
}()

// LookupAttribute returns the AttributeMeta for a given ID, or false if unknown.
func LookupAttribute(id string) (AttributeMeta, bool) {
	norm := strings.ToLower(strings.TrimSpace(id))
	if norm == "brp" {
		norm = "brp_setback"
	}
	if norm == "hb_drop" || norm == "drop" || norm == "hb_rise" || norm == "rise" || norm == "saddle_hb_rise" || norm == "hbrise" {
		norm = "saddle_hb_drop"
	}
	if norm == "stem" || norm == "beststem" || norm == "best-stem" {
		norm = "best_stem"
	}
	if norm == "sit_bones" || norm == "sitbones" || norm == "sitbone" || norm == "sit_bone" {
		norm = "sit_bone_setback"
	}
	if norm == "idealsta" {
		norm = "ideal_sta"
	}
	if norm == "trail_mm" || norm == "trail-mm" || norm == "trailmm" {
		norm = "trail"
	}
	if norm == "cockpit_fit" || norm == "cockpit" || norm == "hb_fit_area" || norm == "clamp_fit" || norm == "hbclampfit" || norm == "clampfit" || norm == "oval_fits" || norm == "ovalfits" {
		norm = "handlebar_clamp_fit"
	}
	if norm == "seat_to_rc" || norm == "seat_axis_rear_axle" || norm == "seat-axis-rear-axle" || norm == "seat-rc" || norm == "seat_rc" || norm == "seatrc" || norm == "saddle_rc" || norm == "saddle-rc" || norm == "saddle_rear_axle" || norm == "saddle-rear-axle" || norm == "saddlerearaxle" || norm == "saddle_axle" || norm == "saddle-axle" || norm == "saddle_to_axle" || norm == "saddle-to-axle" {
		norm = "seat_to_rc"
	}
	if norm == "sparevert" || norm == "seat_axis_ht_drop" || norm == "seat-axis-ht-drop" || norm == "spare_vert" || norm == "spare-vert" || norm == "saddle_ht_drop" || norm == "saddle-ht-drop" || norm == "saddlehtdrop" || norm == "saddle_to_ht_drop" || norm == "steerer_stack" {
		norm = "sparevert"
	}
	cleaned := strings.ReplaceAll(strings.ReplaceAll(norm, "_", ""), "-", "")
	if cleaned == "seatpostext" || cleaned == "seatpost" || cleaned == "postext" || cleaned == "postextension" {
		norm = "seatpost_extension"
	}
	meta, ok := registryMap[norm]
	if !ok {
		meta, ok = registryMap[cleaned]
	}
	return meta, ok
}

// AllAttributes returns a copy of all supported AttributeMeta definitions in standard order.
func AllAttributes() []AttributeMeta {
	copied := make([]AttributeMeta, len(registry))
	copy(copied, registry)
	return copied
}

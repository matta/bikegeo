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
	Extract   func(row model.CanonicalComparisonRow) (float64, bool)
	FormatVal func(val float64, row *model.CanonicalComparisonRow) string
}

var registry = []AttributeMeta{
	// Deprecated: setback measures horizontal distance to the bare seat tube centerline (SaddleHeight * cos(STA)),
	// ignoring seatpost setback and saddle rails. Use brp_setback for BRP-referenced saddle setback.
	{
		ID:       "setback",
		Label:    "Setback",
		Unit:     "mm",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.ActualSetback, true
		},
	},
	// Deprecated: aett measures top-tube reach from the bare seat tube centerline.
	{
		ID:       "aett",
		Label:    "AETT",
		Unit:     "mm",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.Aett, true
		},
	},
	{
		ID:       "sparevert",
		Label:    "Spare Vert",
		Unit:     "mm",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.SpareVerticalStack, true
		},
	},
	{
		ID:       "spareaxial",
		Label:    "Spare Axial",
		Unit:     "mm",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.SpareAxialSteerer, true
		},
	},
	{
		ID:       "seat_to_rc",
		Label:    "Seat-RC",
		Unit:     "mm",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.SeatToRearCenter, true
		},
	},
	{
		ID:       "chainstay",
		Label:    "Chainstay",
		Unit:     "mm",
		Decimals: 0,
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
		ID:       "bbdrop",
		Label:    "BB Drop",
		Unit:     "mm",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.ActiveBbDrop, true
		},
	},
	{
		ID:       "hta",
		Label:    "HTA",
		Unit:     "°",
		Decimals: 1,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.ActiveHta, true
		},
	},
	{
		ID:       "sta",
		Label:    "STA",
		Unit:     "°",
		Decimals: 1,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.ActiveSta, true
		},
	},
	{
		ID:       "reach",
		Label:    "Reach",
		Unit:     "mm",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.ActiveReach, true
		},
	},
	{
		ID:       "stack",
		Label:    "Stack",
		Unit:     "mm",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.ActiveStack, true
		},
	},
	{
		ID:       "wheelbase",
		Label:    "Wheelbase",
		Unit:     "mm",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.WheelbaseCalc, true
		},
	},
	{
		ID:       "frontwt",
		Label:    "Front Wt",
		Unit:     "%",
		Decimals: 1,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.WeightDistFrontPct, true
		},
	},
	{
		ID:       "backangle",
		Label:    "Back Angle",
		Unit:     "°",
		Decimals: 1,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.BackAngleDeg, true
		},
	},
	{
		ID:       "seattube",
		Label:    "Seat Tube",
		Unit:     "mm",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			if row.Bike.SeatTubeLength <= 0 {
				return 0, false
			}
			return row.Bike.SeatTubeLength, true
		},
	},
	{
		ID:       "standover",
		Label:    "Standover",
		Unit:     "mm",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			if row.Bike.StandoverHeight <= 0 {
				return 0, false
			}
			return row.Bike.StandoverHeight, true
		},
	},
	{
		ID:       "brp_setback",
		Label:    "BRP Setback",
		Unit:     "mm",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			if row.Canonical.DeadCenterBRP.BrpSetbackMm <= 0 {
				return 0, false
			}
			return row.Canonical.DeadCenterBRP.BrpSetbackMm, true
		},
		FormatVal: func(val float64, row *model.CanonicalComparisonRow) string {
			if row != nil && row.Canonical.SelectedBRP != nil {
				brp := row.Canonical.SelectedBRP
				sign := "+"
				if brp.RailOffsetMm < 0 {
					sign = ""
				}
				return fmt.Sprintf("%.0f (rail %s%.0fmm)", brp.BrpSetbackMm, sign, brp.RailOffsetMm)
			}
			return fmt.Sprintf("%.0f", val)
		},
	},
	{
		ID:       "saddle_hb",
		Label:    "Saddle-HB",
		Unit:     "mm",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			if row.Canonical.SaddleHbMm <= 0 {
				return 0, false
			}
			return row.Canonical.SaddleHbMm, true
		},
		FormatVal: func(val float64, row *model.CanonicalComparisonRow) string {
			if row != nil && row.Canonical.SelectedHB != nil && row.Canonical.SelectedHB.Stem.LengthMm > 0 {
				hb := row.Canonical.SelectedHB
				sign := "+"
				if hb.SpacerMm < 0 {
					sign = ""
				}
				return fmt.Sprintf("%.0f (%.0fmm %.0f° %s%.0fmm)", val, hb.Stem.LengthMm, hb.Stem.AngleDeg, sign, hb.SpacerMm)
			}
			return fmt.Sprintf("%.0f", val)
		},
	},
	{
		ID:       "saddle_hb_drop",
		Label:    "Saddle-HB Drop",
		Unit:     "mm",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			if row.Canonical.SelectedHB == nil {
				return 0, false
			}
			return row.Canonical.SelectedHB.SaddleHbDropMm, true
		},
		FormatVal: func(val float64, row *model.CanonicalComparisonRow) string {
			sign := ""
			if val > 0 {
				sign = "+"
			}
			return fmt.Sprintf("%s%.0f", sign, val)
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
	if norm == "hb_drop" || norm == "drop" {
		norm = "saddle_hb_drop"
	}
	meta, ok := registryMap[norm]
	if !ok {
		cleaned := strings.ReplaceAll(strings.ReplaceAll(norm, "_", ""), "-", "")
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

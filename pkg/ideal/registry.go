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
	{
		ID:       "setback",
		Label:    "Setback",
		Unit:     "mm",
		Decimals: 0,
		Extract: func(row model.CanonicalComparisonRow) (float64, bool) {
			return row.Canonical.ActualSetback, true
		},
	},
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
}

var registryMap = func() map[string]AttributeMeta {
	m := make(map[string]AttributeMeta, len(registry))
	for _, a := range registry {
		m[strings.ToLower(a.ID)] = a
	}
	return m
}()

// LookupAttribute returns the AttributeMeta for a given ID, or false if unknown.
func LookupAttribute(id string) (AttributeMeta, bool) {
	meta, ok := registryMap[strings.ToLower(strings.TrimSpace(id))]
	return meta, ok
}

// AllAttributes returns a copy of all supported AttributeMeta definitions in standard order.
func AllAttributes() []AttributeMeta {
	copied := make([]AttributeMeta, len(registry))
	copy(copied, registry)
	return copied
}

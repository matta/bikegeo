package ideal

import (
	"bikegeo/pkg/geom2d"
	"bikegeo/pkg/model"
)

// Range specifies an inclusive numeric interval [Min, Max].
// If Min is unset, the range is unbounded below.
// If Max is unset, the range is unbounded above.
type Range struct {
	Min model.Opt[float64]
	Max model.Opt[float64]
}

// Between returns a Range with inclusive [min, max] bounds.
func Between(min, max float64) *Range {
	return &Range{Min: model.Some(min), Max: model.Some(max)}
}

// Exact returns a single-point Range [val, val].
func Exact(val float64) *Range {
	return &Range{Min: model.Some(val), Max: model.Some(val)}
}

// AtLeast returns a lower-bounded Range [min, +inf).
func AtLeast(min float64) *Range {
	return &Range{Min: model.Some(min)}
}

// Cost is a band's price in points of total per unit outside its free range
// (the AttributeSpec Ideal range). Total = 100 − Σ points lost, so each band's
// cost stands on its own: adding or removing a band never rescales the others.
// A zero side is free.
type Cost struct {
	Below float64
	Above float64
}

// CostBoth returns a Cost with separate prices below and above the free range.
func CostBoth(below, above float64) *Cost {
	return &Cost{Below: below, Above: above}
}

// CostEach returns a Cost with the same price on both sides of the free range.
func CostEach(perUnit float64) *Cost {
	return &Cost{Below: perUnit, Above: perUnit}
}

// MetricID identifies a known bike geometry or fit metric in the attribute registry.
type MetricID string

const (
	MetricSetback           MetricID = "setback"
	MetricEtt               MetricID = "ett"
	MetricAett              MetricID = "aett"
	MetricSpareVert         MetricID = "sparevert"
	MetricSpareAxial        MetricID = "spareaxial"
	MetricSeatpostExtension MetricID = "seatpost_extension"
	MetricSeatToRc          MetricID = "seat_to_rc"
	MetricChainstay         MetricID = "chainstay"
	MetricBbDrop            MetricID = "bbdrop"
	MetricHta               MetricID = "hta"
	MetricTrail             MetricID = "trail"
	MetricSta               MetricID = "sta"
	MetricReach             MetricID = "reach"
	MetricStack             MetricID = "stack"
	MetricWheelbase         MetricID = "wheelbase"
	MetricRcWbRatio         MetricID = "rc_wb_ratio"
	MetricSeatTube          MetricID = "seattube"
	MetricStandover         MetricID = "standover"
	MetricBrpSetback        MetricID = "brp_setback"
	MetricSaddleHb          MetricID = "saddle_hb"
	MetricSaddleHbDrop      MetricID = "saddle_hb_drop"
	MetricBestStem          MetricID = "best_stem"
	MetricStemSpacerFit     MetricID = "stem_spacer_fit"
	MetricSitBoneSetback    MetricID = "sit_bone_setback"
	MetricIdealSta          MetricID = "ideal_sta"
	MetricHandlebarClampFit MetricID = "handlebar_clamp_fit"
	MetricRailOffset        MetricID = "rail_offset"
	MetricStemLength        MetricID = "stem_length"
	MetricStemAngle         MetricID = "stem_angle"
	MetricSpacers           MetricID = "spacers"
)

// AttributeSpec describes the scoring and display configuration for a single geometry or fit metric.
type AttributeSpec struct {
	ID MetricID
	// Cost prices the distance outside Ideal (the free range). A nil Cost
	// (and zero LimitPoints) means the attribute is display-only.
	Cost *Cost
	// LimitPoints prices a hardware band: free inside Ideal, then linear up to
	// LimitPoints at the setup's hardware limit on each side (see HardwareLimits).
	// The per-unit cost therefore depends on the row's hardware (stem catalog,
	// saddle rail window, steerer spacer stack).
	LimitPoints float64
	Ideal       *Range
}

// IsScored reports whether the attribute contributes points lost to the total.
func (s AttributeSpec) IsScored() bool {
	return (s.Cost != nil || s.LimitPoints > 0) && s.Ideal != nil
}

// DisplayOnly returns an unscored, display-only AttributeSpec for the given attribute ID.
func DisplayOnly(id MetricID) AttributeSpec {
	return AttributeSpec{ID: id}
}

// Profile represents an idealized frame definition with sweet-spot ranges across attributes.
type Profile struct {
	Name                   string
	Description            string
	SaddleHeightMm         model.Opt[float64]
	TargetSitBoneSetbackMm model.Opt[float64]
	ReferenceSitBones      geom2d.Point2
	ReferenceBrp           geom2d.Point2
	ReferenceSaddle        model.SaddleSpec
	TargetRef              model.TargetRef
	ReferenceBar           model.Opt[model.HandlebarSpec]
	// CockpitTargetDistMm and CockpitTargetRiseMm are the explicit cockpit target
	// for the reference saddle: the hands sit CockpitTargetDistMm from the BRP and
	// CockpitTargetRiseMm above the sit bones. CockpitTarget derives the distance
	// for any other saddle.
	CockpitTargetDistMm float64
	CockpitTargetRiseMm float64
	Attributes          []AttributeSpec
}

// Spec returns the profile's attribute spec for id.
func (p *Profile) Spec(id MetricID) (AttributeSpec, bool) {
	if p == nil {
		return AttributeSpec{}, false
	}
	for _, a := range p.Attributes {
		if a.ID == id {
			return a, true
		}
	}
	return AttributeSpec{}, false
}

// ZoneType defines which region a metric value falls into.
type ZoneType string

const (
	ZoneIdeal   ZoneType = "ideal"
	ZoneOutside ZoneType = "outside"
)

const (
	SymbolIdeal   = "✓"
	SymbolOutside = "✗"
)

// Evaluation contains the computed score and zone classification for a specific metric value.
type Evaluation struct {
	Value           float64
	Zone            ZoneType
	DistanceToIdeal float64
	PointsLost      float64
	FormattedCell   string
	IsDisplayOnly   bool
}

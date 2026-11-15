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

// AtMost returns an upper-bounded Range (-inf, max].
func AtMost(max float64) *Range {
	return &Range{Max: model.Some(max)}
}

// Falloff specifies the drop-off distance outside the acceptable range
// where the sub-score drops from the edge score to 0.
type Falloff struct {
	Below model.Opt[float64]
	Above model.Opt[float64]
}

// FalloffBoth returns a Falloff with both Below and Above drop-off distances.
func FalloffBoth(below, above float64) *Falloff {
	return &Falloff{Below: model.Some(below), Above: model.Some(above)}
}

// FalloffBelow returns a Falloff with only a Below drop-off distance.
func FalloffBelow(below float64) *Falloff {
	return &Falloff{Below: model.Some(below)}
}

// FalloffAbove returns a Falloff with only an Above drop-off distance.
func FalloffAbove(above float64) *Falloff {
	return &Falloff{Above: model.Some(above)}
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
)

// AttributeSpec describes the scoring and display configuration for a single geometry or fit metric.
type AttributeSpec struct {
	ID MetricID
	// Cost prices the distance outside Ideal (the free range). A nil Cost
	// means the attribute is display-only.
	Cost       *Cost
	Ideal      *Range
	Acceptable *Range
	EdgeScore  model.Opt[float64]
	Falloff    *Falloff
	Floor      model.Opt[float64]
}

// IsScored reports whether the attribute contributes points lost to the total.
func (s AttributeSpec) IsScored() bool {
	return s.Cost != nil && s.Ideal != nil
}

// DisplayOnly returns an unscored, display-only AttributeSpec for the given attribute ID.
func DisplayOnly(id MetricID) AttributeSpec {
	return AttributeSpec{ID: id}
}

// DisplayOnlyWithTarget returns an unscored AttributeSpec that displays target ranges without contributing to the score.
func DisplayOnlyWithTarget(id MetricID, ideal, acceptable *Range, edgeScore model.Opt[float64], falloff *Falloff) AttributeSpec {
	return AttributeSpec{
		ID:         id,
		Ideal:      ideal,
		Acceptable: acceptable,
		EdgeScore:  edgeScore,
		Falloff:    falloff,
	}
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
	Attributes             []AttributeSpec
}

// ProfileSummary provides high-level metadata for listing profiles.
type ProfileSummary struct {
	Name           string
	Description    string
	SaddleHeightMm model.Opt[float64]
	TargetRef      model.TargetRef
	AttributeCount int
	ScoredCount    int
}

// ZoneType defines which region a metric value falls into.
type ZoneType string

const (
	ZoneIdeal      ZoneType = "ideal"
	ZoneAcceptable ZoneType = "acceptable"
	ZoneOutside    ZoneType = "outside"
)

const (
	SymbolIdeal      = "✓"
	SymbolAcceptable = "~"
	SymbolOutside    = "✗"
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

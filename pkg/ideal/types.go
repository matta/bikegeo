package ideal

// Float returns a pointer to v.
func Float(v float64) *float64 {
	return &v
}

// Range specifies an inclusive numeric interval [Min, Max].
// If Min is nil, the range is unbounded below.
// If Max is nil, the range is unbounded above.
type Range struct {
	Min *float64
	Max *float64
}

// Between returns a Range with inclusive [min, max] bounds.
func Between(min, max float64) *Range {
	return &Range{Min: Float(min), Max: Float(max)}
}

// Exact returns a single-point Range [val, val].
func Exact(val float64) *Range {
	return &Range{Min: Float(val), Max: Float(val)}
}

// AtLeast returns a lower-bounded Range [min, +inf).
func AtLeast(min float64) *Range {
	return &Range{Min: Float(min)}
}

// AtMost returns an upper-bounded Range (-inf, max].
func AtMost(max float64) *Range {
	return &Range{Max: Float(max)}
}

// Falloff specifies the drop-off distance outside the acceptable range
// where the sub-score drops from the edge score to 0.
type Falloff struct {
	Below *float64
	Above *float64
}

// FalloffBoth returns a Falloff with both Below and Above drop-off distances.
func FalloffBoth(below, above float64) *Falloff {
	return &Falloff{Below: Float(below), Above: Float(above)}
}

// FalloffBelow returns a Falloff with only a Below drop-off distance.
func FalloffBelow(below float64) *Falloff {
	return &Falloff{Below: Float(below)}
}

// FalloffAbove returns a Falloff with only an Above drop-off distance.
func FalloffAbove(above float64) *Falloff {
	return &Falloff{Above: Float(above)}
}

// AttributeSpec describes the scoring and display configuration for a single geometry or fit metric.
type AttributeSpec struct {
	ID         string
	Weight     float64
	Ideal      *Range
	Acceptable *Range
	EdgeScore  *float64
	Falloff    *Falloff
	Floor      *float64
}

// DisplayOnly returns an unscored, display-only AttributeSpec for the given attribute ID.
func DisplayOnly(id string) AttributeSpec {
	return AttributeSpec{ID: id}
}

// Profile represents an idealized frame definition with sweet-spot ranges across attributes.
type Profile struct {
	Name           string
	Description    string
	SaddleHeightMm *float64
	Attributes     []AttributeSpec
}

// ProfileSummary provides high-level metadata for listing profiles.
type ProfileSummary struct {
	Name           string
	Description    string
	SaddleHeightMm *float64
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
	SubScore        float64
	FormattedCell   string
	IsDisplayOnly   bool
}

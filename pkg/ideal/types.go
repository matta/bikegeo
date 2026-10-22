package ideal

// Range specifies an inclusive numeric interval [Min, Max].
// If Min is nil, the range is unbounded below.
// If Max is nil, the range is unbounded above.
type Range struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

// Falloff specifies the drop-off distance outside the acceptable range
// where the sub-score drops from the edge score to 0.
type Falloff struct {
	Below *float64 `json:"below,omitempty"`
	Above *float64 `json:"above,omitempty"`
}

// AttributeSpec describes the scoring and display configuration for a single geometry or fit metric.
type AttributeSpec struct {
	ID         string   `json:"id"`
	Weight     float64  `json:"weight,omitempty"`
	Ideal      *Range   `json:"ideal,omitempty"`
	Acceptable *Range   `json:"acceptable,omitempty"`
	EdgeScore  *float64 `json:"edge_score,omitempty"`
	Falloff    *Falloff `json:"falloff,omitempty"`
	Floor      *float64 `json:"floor,omitempty"`
}

// Profile represents an idealized frame definition with sweet-spot ranges across attributes.
type Profile struct {
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	SaddleHeightMm *float64        `json:"saddle_height_mm,omitempty"`
	Attributes     []AttributeSpec `json:"attributes"`
}

// ProfileSummary provides high-level metadata for listing profiles.
type ProfileSummary struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	SaddleHeightMm *float64 `json:"saddle_height_mm,omitempty"`
	AttributeCount int      `json:"attribute_count"`
	ScoredCount    int      `json:"scored_count"`
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

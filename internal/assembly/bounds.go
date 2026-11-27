package assembly

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
)

// BoundedRange is a validated closed interval [min, max] in millimeters with
// provenance labels identifying the physical rule behind each bound.
// The zero value is invalid; construct instances with NewBoundedRange.
type BoundedRange struct {
	min, max             float64
	minSource, maxSource string
	valid                bool
}

// NewBoundedRange constructs a validated [min, max] interval. Endpoints must
// not be NaN, min must not be +Inf, max must not be -Inf, and min must not
// exceed max. Infinite endpoints (-Inf for min, +Inf for max) are permitted
// for one-sided solver domains.
func NewBoundedRange(minVal, maxVal float64, minSource, maxSource string) (BoundedRange, error) {
	if math.IsNaN(minVal) || math.IsNaN(maxVal) {
		return BoundedRange{}, fmt.Errorf("bounded range endpoints must not be NaN, got [%v, %v]", minVal, maxVal)
	}
	if math.IsInf(minVal, 1) || math.IsInf(maxVal, -1) {
		return BoundedRange{}, fmt.Errorf("bounded range endpoints are degenerate: [%v, %v]", minVal, maxVal)
	}
	if minVal > maxVal {
		return BoundedRange{}, fmt.Errorf("bounded range minimum %v exceeds maximum %v", minVal, maxVal)
	}
	// Normalize -0 to +0 for clean display and equality checks.
	if minVal == 0 {
		minVal = 0
	}
	if maxVal == 0 {
		maxVal = 0
	}
	return BoundedRange{
		min:       minVal,
		max:       maxVal,
		minSource: minSource,
		maxSource: maxSource,
		valid:     true,
	}, nil
}

func (r BoundedRange) requireValid() {
	if !r.valid {
		panic("assembly: uninitialized zero-value BoundedRange")
	}
}

// Valid reports whether r was constructed via NewBoundedRange or IntersectRanges.
func (r BoundedRange) Valid() bool {
	return r.valid
}

// Min returns the lower bound of r in millimeters.
func (r BoundedRange) Min() float64 {
	r.requireValid()
	return r.min
}

// Max returns the upper bound of r in millimeters.
func (r BoundedRange) Max() float64 {
	r.requireValid()
	return r.max
}

// MinSource returns the provenance label for the lower bound.
func (r BoundedRange) MinSource() string {
	r.requireValid()
	return r.minSource
}

// MaxSource returns the provenance label for the upper bound.
func (r BoundedRange) MaxSource() string {
	r.requireValid()
	return r.maxSource
}

// Midpoint returns the midpoint of [Min, Max]. For an interval unbounded above
// (+Inf) or below (-Inf), it returns that infinity.
func (r BoundedRange) Midpoint() float64 {
	r.requireValid()
	if math.IsInf(r.min, -1) && math.IsInf(r.max, 1) {
		return 0
	}
	if math.IsInf(r.min, -1) {
		return math.Inf(-1)
	}
	if math.IsInf(r.max, 1) {
		return math.Inf(1)
	}
	if r.min < 0 && r.max > 0 {
		return (r.min + r.max) / 2.0
	}
	return r.min + (r.max-r.min)/2.0
}

// Clamp restricts v to [Min, Max]. NaN propagates unchanged.
func (r BoundedRange) Clamp(v float64) float64 {
	r.requireValid()
	if math.IsNaN(v) {
		return v
	}
	if v < r.min {
		return r.min
	}
	if v > r.max {
		return r.max
	}
	return v
}

// Bind validates that v is finite and lies within [Min, Max], returning a
// BoundStatus readout or an *InputError attributed to field.
func (r BoundedRange) Bind(field InputField, v float64) (BoundStatus, error) {
	r.requireValid()
	if !isFinite(v) {
		return BoundStatus{}, invalidInput(field, "%s must be finite, got %v mm", field, v)
	}
	if v < r.min || v > r.max {
		return BoundStatus{}, invalidInput(field, "%s must be within %v–%v mm, got %v mm", field, r.min, r.max, v)
	}
	return BoundStatus{
		Field:     field,
		Value:     v,
		Min:       r.min,
		Max:       r.max,
		MinSource: r.minSource,
		MaxSource: r.maxSource,
	}, nil
}

// IntersectRanges returns the intersection of two BoundedRanges, picking the
// tighter endpoint on each side and preserving that endpoint's provenance
// label. It returns false when either range is invalid or the intersection is
// empty.
func IntersectRanges(a, b BoundedRange) (BoundedRange, bool) {
	if !a.valid || !b.valid {
		return BoundedRange{}, false
	}
	lo, loSrc := a.min, a.minSource
	if b.min > lo || (b.min == lo && loSrc == "") {
		lo, loSrc = b.min, b.minSource
	}
	hi, hiSrc := a.max, a.maxSource
	if b.max < hi || (b.max == hi && hiSrc == "") {
		hi, hiSrc = b.max, b.maxSource
	}
	if lo > hi {
		return BoundedRange{}, false
	}
	return BoundedRange{
		min:       lo,
		max:       hi,
		minSource: loSrc,
		maxSource: hiSrc,
		valid:     true,
	}, true
}

// BoundStatus is an exported readout snapshot recording a bound value together
// with the physical limits and provenance labels that constrained it. Unbounded
// endpoints (-Inf for Min, +Inf for Max) serialize as JSON null and unmarshal
// back to -Inf / +Inf.
type BoundStatus struct {
	Field     InputField
	Value     float64
	Min       float64
	Max       float64
	MinSource string
	MaxSource string
}

type boundStatusJSON struct {
	Field     InputField
	Value     float64
	Min       *float64
	Max       *float64
	MinSource string
	MaxSource string
}

// MarshalJSON encodes b with -Inf Min and +Inf Max represented as JSON null so
// one-sided or unbounded intervals serialize cleanly for CLI output.
func (b BoundStatus) MarshalJSON() ([]byte, error) {
	var minPtr, maxPtr *float64
	switch {
	case math.IsInf(b.Min, -1):
		minPtr = nil
	case !isFinite(b.Min):
		return nil, fmt.Errorf("assembly: BoundStatus.Min must be finite or -Inf, got %v", b.Min)
	default:
		v := b.Min
		minPtr = &v
	}
	switch {
	case math.IsInf(b.Max, 1):
		maxPtr = nil
	case !isFinite(b.Max):
		return nil, fmt.Errorf("assembly: BoundStatus.Max must be finite or +Inf, got %v", b.Max)
	default:
		v := b.Max
		maxPtr = &v
	}
	return json.Marshal(boundStatusJSON{
		Field:     b.Field,
		Value:     b.Value,
		Min:       minPtr,
		Max:       maxPtr,
		MinSource: b.MinSource,
		MaxSource: b.MaxSource,
	})
}

// UnmarshalJSON decodes a JSON-encoded BoundStatus, restoring null Min/Max
// endpoints to -Inf / +Inf.
func (b *BoundStatus) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}
	var wire boundStatusJSON
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	minVal := math.Inf(-1)
	if wire.Min != nil {
		minVal = *wire.Min
	}
	maxVal := math.Inf(1)
	if wire.Max != nil {
		maxVal = *wire.Max
	}
	*b = BoundStatus{
		Field:     wire.Field,
		Value:     wire.Value,
		Min:       minVal,
		Max:       maxVal,
		MinSource: wire.MinSource,
		MaxSource: wire.MaxSource,
	}
	return nil
}

// MarginMm returns the smaller clearance in millimeters from Value to Min or Max.
func (b BoundStatus) MarginMm() float64 {
	return min(b.Value-b.Min, b.Max-b.Value)
}

// WithinMargin reports whether Value lies within mm of either bound
// (MarginMm() <= mm).
func (b BoundStatus) WithinMargin(mm float64) bool {
	return b.MarginMm() <= mm
}

// AtMin reports whether Value sits within tol of Min.
func (b BoundStatus) AtMin(tol float64) bool {
	return math.Abs(b.Value-b.Min) <= tol
}

// AtMax reports whether Value sits within tol of Max.
func (b BoundStatus) AtMax(tol float64) bool {
	return math.Abs(b.Value-b.Max) <= tol
}
